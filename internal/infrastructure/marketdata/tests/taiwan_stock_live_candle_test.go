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

// fugleIntradaySourceUnderTest serves the next set of candles per round, repeating the last once exhausted like the real venue between minutes.
type fugleIntradaySourceUnderTest struct {
	server     *httptest.Server
	rounds     [][]string
	statusCode int
	rawBody    string
	mutex      sync.Mutex
	// roundIndex advances when a symbol comes round again, so the fake needs no symbol count.
	roundIndex    int
	servedInRound map[string]bool
	askedSymbols  []string
}

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

// A later minute proves the earlier one finished, and the stored minute is the venue's complete version, not the part the follow witnessed.
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
	// 500, not the 344 seen when the follow joined.
	assert.Equal(t, "500", finished.Volume.String())
	assert.Equal(t, "112.95", finished.Close.String())

	started := nextKCandle(t, liveKCandles)
	assert.False(t, started.Closed)
	assert.Equal(t, "120", started.Volume.String())
}

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

func TestASymbolWithNoCandlesYetYieldsNothing(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t, []string{})

	liveKCandles, _ := followFugleIntraday(t, source, "1418")

	select {
	case liveKCandle, isOpen := <-liveKCandles:
		assert.False(t, isOpen, "a symbol with no candles must not produce one: %v", liveKCandle)
	case <-time.After(300 * time.Millisecond):
	}
}

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

func TestAFollowSlowsItselfToWhatTheVenueAllowanceAffords(t *testing.T) {
	source := newFugleIntradaySourceUnderTest(t,
		[]string{fugleIntradayCandle("09:30", "112.9", "344")})

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	// Sixty a minute across six symbols allows at most one round every six seconds.
	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 60, time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext, vo.NewLiveFollowChannelVo(
		vo.MarketTaiwanStock, []string{"1", "2", "3", "4", "5", "6"}))
	require.NoError(t, followError)

	for range 6 {
		nextKCandle(t, liveKCandles)
	}

	// Only the opening round can have happened; the next is six seconds away.
	time.Sleep(300 * time.Millisecond)
	assert.Len(t, source.symbolsAsked(), 6)
}

// contextWithCancel is cancelled at test end so no follow leaks into the next test.
func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	return executionContext, stopFollowing
}

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

// Cancellation must end a parked follow immediately, not at its next round.
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

// Cancellation must release a send blocked on a full channel, or the goroutine leaks.
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
	// Sixty symbols at sixty a minute is one round a minute, longer than the quiet timeout.
	_, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		source.server.URL, "a-key", time.Millisecond, 60, 30*time.Second,
		2*time.Second, marketdata.NewRequestPacer(0),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, manySymbols))
	require.NoError(t, followError)

	assert.Contains(t, writtenDown.text(), "longer than",
		"a follow that cannot beat the silence must say so")
}

// lockedLogBuffer is written by the follow's goroutine and read by the test, so it needs a lock.
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
