package marketdata_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fugleIntradaySourceUnderTest answers each request with the next set of candles the
// test handed it, repeating the last once it runs out — which is what the real venue
// does between one minute and the next.
type fugleIntradaySourceUnderTest struct {
	server     *httptest.Server
	rounds     [][]string
	statusCode int
	rawBody    string
	mutex      sync.Mutex
	// roundIndex is which set of candles is being served. A round is over when a
	// symbol comes round again, which is how the fake knows a poll finished without
	// being told how many symbols are on the channel.
	roundIndex    int
	servedInRound map[string]bool
	askedSymbols  []string
}

// fugleIntradayCandle spells one candle the way this venue does.
func fugleIntradayCandle(localTime string, closePrice string, volume string) string {
	return fmt.Sprintf(
		`{"date":"2026-09-23T%s:00.000+08:00","open":112.75,"high":113,"low":112.5,`+
			`"close":%s,"volume":%s,"average":112.9}`, localTime, closePrice, volume)
}

func newFugleIntradaySourceUnderTest(
	t *testing.T, rounds ...[]string,
) *fugleIntradaySourceUnderTest {
	t.Helper()

	source := &fugleIntradaySourceUnderTest{
		rounds: rounds, statusCode: http.StatusOK, servedInRound: map[string]bool{},
	}
	source.server = httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter, request *http.Request,
	) {
		source.mutex.Lock()
		symbol := request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
		source.askedSymbols = append(source.askedSymbols, symbol)

		if source.servedInRound[symbol] {
			source.roundIndex++
			source.servedInRound = map[string]bool{}
		}
		source.servedInRound[symbol] = true

		roundIndex := min(source.roundIndex, len(source.rounds)-1)
		statusCode, rawBody := source.statusCode, source.rawBody
		source.mutex.Unlock()

		if statusCode != http.StatusOK {
			responseWriter.WriteHeader(statusCode)

			return
		}
		if rawBody != "" {
			_, _ = responseWriter.Write([]byte(rawBody))

			return
		}

		candles := []string{}
		if len(source.rounds) > 0 && roundIndex >= 0 {
			candles = source.rounds[roundIndex]
		}
		_, _ = responseWriter.Write([]byte(fmt.Sprintf(
			`{"symbol":"%s","data":[%s]}`, symbol, strings.Join(candles, ","))))
	}))
	t.Cleanup(source.server.Close)

	return source
}

func (source *fugleIntradaySourceUnderTest) symbolsAsked() []string {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	return append([]string(nil), source.askedSymbols...)
}

func followFugleIntraday(
	t *testing.T, source *fugleIntradaySourceUnderTest, symbols ...string,
) (<-chan vo.LiveKCandleVo, context.CancelFunc) {
	t.Helper()

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 0, time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, symbols))
	require.NoError(t, followError)

	return liveKCandles, stopFollowing
}

func nextKCandle(t *testing.T, liveKCandles <-chan vo.LiveKCandleVo) vo.LiveKCandleVo {
	t.Helper()

	select {
	case liveKCandle, isOpen := <-liveKCandles:
		require.True(t, isOpen, "the feed ended before reporting a candle")

		return liveKCandle
	case <-time.After(3 * time.Second):
		t.Fatal("no candle arrived")

		return vo.LiveKCandleVo{}
	}
}

// The venue states the minute already formed, so the figures are carried across as
// they arrive — no folding, no differencing, nothing invented.
func TestTheLatestMinuteIsReportedAsItStands(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	liveKCandles, _ := followFugleIntraday(t, source, "0050")

	liveKCandle := nextKCandle(t, liveKCandles)
	assert.Equal(t, "0050", liveKCandle.Symbol)
	assert.Equal(t, "344", liveKCandle.Volume.String())
	assert.Equal(t, "112.9", liveKCandle.Close.String())
	assert.False(t, liveKCandle.Closed, "the latest minute is still forming")
}

// A later minute appearing is what proves the earlier one finished, and the venue has
// already stated that earlier minute whole — so what gets stored is the complete
// minute, never the part of it that happened to be seen.
//
// This is the difference that matters. A feed assembled from quotes can only ever
// store the part of a minute it witnessed, which is why the minute a follow opens in
// used to be wrong and permanent.
func TestAFinishedMinuteIsStoredWholeEvenIfTheFollowJoinedPartWayThroughIt(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")},
		[]string{
			fugleIntradayCandle("09:30", "112.95", "500"),
			fugleIntradayCandle("09:31", "113", "120"),
		})

	liveKCandles, _ := followFugleIntraday(t, source, "0050")

	forming := nextKCandle(t, liveKCandles)
	require.False(t, forming.Closed)

	finished := nextKCandle(t, liveKCandles)
	assert.True(t, finished.Closed)
	// 500, not the 344 that minute had been up to when the follow joined it.
	assert.Equal(t, "500", finished.Volume.String())
	assert.Equal(t, "112.95", finished.Close.String())

	started := nextKCandle(t, liveKCandles)
	assert.False(t, started.Closed)
	assert.Equal(t, "120", started.Volume.String())
}

// Every symbol on the channel is asked about, one request each.
func TestEverySymbolOnTheChannelIsAskedAbout(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	liveKCandles, _ := followFugleIntraday(t, source, "0050", "2330")
	nextKCandle(t, liveKCandles)

	require.Eventually(t, func() bool {
		asked := source.symbolsAsked()

		return len(asked) >= 2
	}, 3*time.Second, 10*time.Millisecond)

	assert.Subset(t, source.symbolsAsked(), []string{"0050", "2330"})
}

// A symbol with nothing today yields nothing, and does not stop the others.
func TestASymbolWithNoCandlesYetYieldsNothing(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t, []string{})

	liveKCandles, _ := followFugleIntraday(t, source, "1418")

	select {
	case liveKCandle, isOpen := <-liveKCandles:
		assert.False(t, isOpen, "a symbol with no candles must not produce one: %v", liveKCandle)
	case <-time.After(300 * time.Millisecond):
	}
}

// A venue that will not answer is a failure to open, not a feed that opened and went
// quiet — and the two lead the caller to do different things.
func TestAVenueThatWillNotAnswerNeverHandsBackAFeed(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		rawBody    string
	}{
		{name: "turned away", statusCode: http.StatusTooManyRequests},
		{name: "an answer that is not an answer", statusCode: http.StatusOK, rawBody: "{nope"},
		{
			name:       "a good answer with junk after it",
			statusCode: http.StatusOK,
			rawBody:    `{"symbol":"0050","data":[]} and then some`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newFugleIntradaySourceUnderTest(t, []string{})
			source.statusCode = testCase.statusCode
			source.rawBody = testCase.rawBody

			executionContext, stopFollowing := context.WithCancel(context.Background())
			t.Cleanup(stopFollowing)

			liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
				source.server.URL, "a-key", time.Millisecond, 0, time.Hour,
				2*time.Second, marketdata.NewRequestPacer(0),
			).FollowKCandles(executionContext,
				vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"0050"}))

			assert.Error(t, followError)
			assert.Nil(t, liveKCandles)
		})
	}
}

// Ending a follow closes the feed, which is the one way a caller learns it is over.
func TestEndingAFugleIntradayFollowClosesTheFeed(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	liveKCandles, stopFollowing := followFugleIntraday(t, source, "0050")
	stopFollowing()

	require.Eventually(t, func() bool {
		for {
			select {
			case _, isOpen := <-liveKCandles:
				if !isOpen {
					return true
				}
			default:
				return false
			}
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// One request buys one symbol, so a long watchlist cannot be asked about as often as
// a short one. Working that out up front turns a silent slowdown into a number.
func TestAFollowSlowsItselfToWhatTheVenueAllowanceAffords(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	// Sixty a minute against six symbols is one round every six seconds at best, so a
	// round every millisecond is not on offer however it is configured.
	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 60, time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext, vo.NewLiveFollowChannelVo(
		vo.MarketTaiwanStock, []string{"1", "2", "3", "4", "5", "6"}))
	require.NoError(t, followError)

	for range 6 {
		nextKCandle(t, liveKCandles)
	}

	// Six symbols asked once each opening the follow; a second round cannot have
	// happened yet, because it is not affordable for another six seconds.
	time.Sleep(300 * time.Millisecond)
	assert.Len(t, source.symbolsAsked(), 6)
}

// contextWithCancel is a cancellable context that is always let go of when the test
// ends, so a follow started by one test never outlives it into the next.
func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	return executionContext, stopFollowing
}

// A date the venue states in a way that cannot be read ends the round rather than
// becoming a candle at some arbitrary moment.
func TestACandleWithAnUnreadableDateEndsTheRound(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t, []string{})
	source.rawBody = `{"symbol":"0050","data":[{"date":"half past nine","open":1,` +
		`"high":1,"low":1,"close":1,"volume":1,"average":1}]}`

	executionContext, _ := contextWithCancel(t)
	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 0, time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"0050"}))

	assert.Error(t, followError)
	assert.Nil(t, liveKCandles)
}

// An address that is not an address, and a follow asked for after the work was called
// off, are both refused before anything reaches the network.
func TestAFollowThatCannotEvenBeAttemptedIsRefusedAtOnce(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	calledOff, stopImmediately := context.WithCancel(context.Background())
	stopImmediately()

	testCases := []struct {
		name             string
		sourceUrl        string
		executionContext context.Context
		pacer            marketdata.RequestPacer
	}{
		{
			name: "an address that cannot be read", sourceUrl: "://not-an-address",
			executionContext: context.Background(), pacer: marketdata.NewRequestPacer(0),
		},
		{
			name: "the work was called off first", sourceUrl: source.server.URL,
			executionContext: calledOff, pacer: marketdata.NewRequestPacer(60),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
				testCase.sourceUrl, "a-key", time.Millisecond, 0, time.Hour,
				2*time.Second, testCase.pacer,
			).FollowKCandles(testCase.executionContext,
				vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"0050"}))

			assert.Error(t, followError)
			assert.Nil(t, liveKCandles)
		})
	}
}

// Between rounds the follow is parked. Ending it there has to end it now rather than
// at the next round — a follow asking once a minute would otherwise outlive its
// cancellation for most of a minute, and every roster rebuild would leave one behind.
func TestEndingAFollowWaitingForItsNextRoundEndsItAtOnce(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Hour, 0, 2*time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"0050"}))
	require.NoError(t, followError)

	stopFollowing()

	require.Eventually(t, func() bool {
		for {
			select {
			case _, isOpen := <-liveKCandles:
				if !isOpen {
					return true
				}
			default:
				return false
			}
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// More candles than the feed will hold, and nobody reading them. Ending the follow has
// to release the send that is waiting for room, or the goroutine behind every
// abandoned follow stays parked on a channel nobody will ever read.
func TestEndingAFugleIntradayFollowReleasesASendWaitingForRoom(t *testing.T) {
	crowdedSymbols := make([]string, 0, 30)
	for symbolIndex := range 30 {
		crowdedSymbols = append(crowdedSymbols, fmt.Sprintf("10%02d", symbolIndex))
	}

	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	liveKCandles, stopFollowing := followFugleIntraday(t, source, crowdedSymbols...)

	time.Sleep(50 * time.Millisecond)
	stopFollowing()

	require.Eventually(t, func() bool {
		for {
			select {
			case _, isOpen := <-liveKCandles:
				if !isOpen {
					return true
				}
			default:
				return false
			}
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// A watchlist the venue's allowance cannot keep up with is said out loud, because from
// every other angle it looks like a market that has gone quiet: the follow keeps being
// given up for dead and reopened, and nothing about the feed is actually wrong.
func TestAWatchlistTooLongForTheAllowanceIsSaidOutLoud(t *testing.T) {
	writtenDown := &lockedLogBuffer{}
	log.SetOutput(writtenDown)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	manySymbols := make([]string, 0, 60)
	for symbolIndex := range 60 {
		manySymbols = append(manySymbols, fmt.Sprintf("10%02d", symbolIndex))
	}

	executionContext, _ := contextWithCancel(t)
	// Sixty symbols against sixty a minute is a round a minute, which no caller
	// waiting thirty seconds for a sign of life will sit through.
	_, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 60, 30*time.Second,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, manySymbols))
	require.NoError(t, followError)

	assert.Contains(t, writtenDown.text(), "longer than",
		"a follow that cannot beat the silence must say so")
}

// lockedLogBuffer collects what was written down while a follow is running. The follow
// writes from its own goroutine and the test reads from this one, so the two have to
// be kept apart — a plain buffer here is a data race, not a shortcut.
type lockedLogBuffer struct {
	mutex   sync.Mutex
	written bytes.Buffer
}

func (lockedLogBuffer *lockedLogBuffer) Write(entry []byte) (int, error) {
	lockedLogBuffer.mutex.Lock()
	defer lockedLogBuffer.mutex.Unlock()

	return lockedLogBuffer.written.Write(entry)
}

func (lockedLogBuffer *lockedLogBuffer) text() string {
	lockedLogBuffer.mutex.Lock()
	defer lockedLogBuffer.mutex.Unlock()

	return lockedLogBuffer.written.String()
}
