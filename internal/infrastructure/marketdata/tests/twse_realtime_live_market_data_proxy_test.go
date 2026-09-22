package marketdata_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func followTwseExpectingFailure(
	t *testing.T, source *twseSourceUnderTest, symbols ...string,
) error {
	t.Helper()

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	proxy := marketdata.NewTwseRealtimeLiveMarketDataProxy(
		source.server.URL, twseTaiwanStockMarket(), time.Millisecond,
		2*time.Second, marketdata.NewRequestPacer(0))

	liveKCandles, followError := proxy.FollowKCandles(
		executionContext, vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, symbols))
	assert.Nil(t, liveKCandles, "no feed may be handed back when opening it failed")

	return followError
}

// Which board a code is listed on cannot be worked out from the code, and the system
// does not record it — so every symbol is asked for under both, in one request.
func TestEverySymbolIsAskedForUnderBothBoards(t *testing.T) {
	source := newTwseSourceUnderTest(t, []twseQuote{quoteAt("10:00:05", "2500", "500")})

	_, _ = followTwse(t, source, "2330", "6488")

	requestedExCh := <-source.requestedExChs
	askedSymbols := strings.Split(requestedExCh, "|")

	assert.ElementsMatch(t,
		[]string{"tse_2330.tw", "otc_2330.tw", "tse_6488.tw", "otc_6488.tw"},
		askedSymbols)
}

// The board that does not list a code answers with an entry carrying nothing. That is
// an answer about where the stock is listed, not a source that cannot be read.
func TestAnEntryFromTheWrongBoardIsIgnoredRatherThanTreatedAsAFailure(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{{}, quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{{}, quoteAt("10:00:45", "2510", "530")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	assert.Equal(t, "30000", nextKCandle(t, liveKCandles).Volume.String())
}

// A stock that has not traded today answers with a dash rather than a zero. Reading
// that as a price would put a candle at zero on somebody's chart.
func TestAStockThatHasNotTradedYetIsPassedOverQuietly(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{
			{Symbol: "2454", LatestPrice: "-", CumulativeVolume: "0",
				LatestTradeTime: "00:00:00", TradingDate: "20260922"},
			quoteAt("10:00:05", "2500", "500"),
		},
		[]twseQuote{
			{Symbol: "2454", LatestPrice: "-", CumulativeVolume: "0",
				LatestTradeTime: "00:00:00", TradingDate: "20260922"},
			quoteAt("10:00:45", "2510", "530"),
		},
	)

	liveKCandles, _ := followTwse(t, source, "2330", "2454")

	liveKCandle := nextKCandle(t, liveKCandles)
	assert.Equal(t, "2330", liveKCandle.Symbol)
	assert.Equal(t, "30000", liveKCandle.Volume.String())
}

// A quote for something this channel never asked for belongs to nobody here.
func TestAQuoteForAnUnfollowedSymbolIsDiscarded(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{
			{Symbol: "9999", LatestPrice: "100", CumulativeVolume: "500",
				LatestTradeTime: "10:00:05", TradingDate: "20260922"},
			quoteAt("10:00:05", "2500", "500"),
		},
		[]twseQuote{
			{Symbol: "9999", LatestPrice: "110", CumulativeVolume: "900",
				LatestTradeTime: "10:00:45", TradingDate: "20260922"},
			quoteAt("10:00:45", "2510", "530"),
		},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	assert.Equal(t, "2330", nextKCandle(t, liveKCandles).Symbol)
}

// A source refusing from the outset is a failure to open, not a feed that opened and
// went quiet — and the two lead the caller to do different things.
func TestASourceThatWillNotAnswerNeverHandsBackAFeed(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		returnCode string
	}{
		{name: "refused outright", statusCode: http.StatusTooManyRequests, returnCode: "0000"},
		{name: "refused inside a perfectly ordinary body", statusCode: http.StatusOK, returnCode: "5001"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newTwseSourceUnderTest(t,
				[]twseQuote{quoteAt("10:00:05", "2500", "500")})
			source.statusCode = testCase.statusCode
			source.returnCode = testCase.returnCode

			assert.Error(t, followTwseExpectingFailure(t, source, "2330"))
		})
	}
}

// Ending a follow closes the feed, which is the one way a caller ever learns that it
// is over.
func TestEndingAFollowClosesTheFeed(t *testing.T) {
	source := newTwseSourceUnderTest(t, []twseQuote{quoteAt("10:00:05", "2500", "500")})

	liveKCandles, stopFollowing := followTwse(t, source, "2330")
	stopFollowing()

	require.Eventually(t, func() bool {
		select {
		case _, isOpen := <-liveKCandles:
			return !isOpen
		default:
			return false
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// Each symbol's minutes are folded on their own. Sharing one folder would let two
// symbols answering alternately keep resetting each other's minute.
func TestSymbolsOnOneChannelAreFoldedIndependently(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{
			quoteAt("10:00:05", "2500", "500"),
			{Symbol: "2454", LatestPrice: "1200", CumulativeVolume: "80",
				LatestTradeTime: "10:00:06", TradingDate: "20260922"},
		},
		[]twseQuote{
			quoteAt("10:00:45", "2510", "530"),
			{Symbol: "2454", LatestPrice: "1210", CumulativeVolume: "95",
				LatestTradeTime: "10:00:46", TradingDate: "20260922"},
		},
	)

	liveKCandles, _ := followTwse(t, source, "2330", "2454")

	volumeBySymbol := map[string]string{}
	for range 2 {
		liveKCandle := nextKCandle(t, liveKCandles)
		volumeBySymbol[liveKCandle.Symbol] = liveKCandle.Volume.String()
	}

	assert.Equal(t, map[string]string{"2330": "30000", "2454": "15000"}, volumeBySymbol)
}

// A stock that has not traded yet answers with a dash and a zero, timed at midnight.
// Taking that for a real quote would open a minute at midnight measuring from zero —
// and then the stock's first real trade of the day would arrive as a minute carrying
// the entire day's volume.
//
// That is the failure the dash exists to prevent, and it is worth stating separately
// from "no candle appears": no candle appears either way. What differs is the first
// candle that does.
func TestAnUntradedQuoteDoesNotBecomeTheBaselineForTheFirstRealTrade(t *testing.T) {
	notTradedYet := twseQuote{
		Symbol: "2454", LatestPrice: "-", CumulativeVolume: "0",
		LatestTradeTime: "00:00:00", TradingDate: "20260922",
	}
	traded := func(tradeTime string, price string, lots string) twseQuote {
		return twseQuote{
			Symbol: "2454", LatestPrice: price, CumulativeVolume: lots,
			LatestTradeTime: tradeTime, TradingDate: "20260922",
		}
	}

	source := newTwseSourceUnderTest(t,
		[]twseQuote{notTradedYet},
		[]twseQuote{traded("10:05:10", "1200", "500")},
		[]twseQuote{traded("10:05:40", "1210", "515")},
	)

	liveKCandles, _ := followTwse(t, source, "2454")

	// Fifteen lots traded inside that minute, not the five hundred the day had.
	assert.Equal(t, "15000", nextKCandle(t, liveKCandles).Volume.String())
}

// A source that cannot be read is not a market with nothing to report. Both would
// otherwise arrive as an empty answer, and one of them means the follow is over.
func TestASourceThatCannotBeReadDoesNotOpenAFeed(t *testing.T) {
	testCases := []struct {
		name    string
		rawBody string
	}{
		{name: "an answer that is not an answer", rawBody: "{not json at all"},
		{
			name:    "a good answer with junk after it",
			rawBody: `{"msgArray":[],"rtcode":"0000","rtmessage":"OK"} and then some`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newTwseSourceUnderTest(t)
			source.rawBody = testCase.rawBody

			assert.Error(t, followTwseExpectingFailure(t, source, "2330"))
		})
	}
}

// A feed that opened and then stopped being answered is over, and closing the channel
// is the one way the caller is told. It must not hang on waiting for an answer that
// is not coming — the retry rules live with the caller, not here.
func TestAFeedThatStopsBeingAnsweredCloses(t *testing.T) {
	source := newTwseSourceUnderTest(t, []twseQuote{quoteAt("10:00:05", "2500", "500")})
	source.failsAfter = 1

	liveKCandles, _ := followTwse(t, source, "2330")

	require.Eventually(t, func() bool {
		select {
		case _, isOpen := <-liveKCandles:
			return !isOpen
		default:
			return false
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// A follow asked for with the work already called off never reaches the source at
// all. Opening a feed for a caller that has gone would spend an allowance nobody is
// waiting on the answer to.
func TestAFollowAskedForAfterTheWorkWasCalledOffNeverOpens(t *testing.T) {
	source := newTwseSourceUnderTest(t, []twseQuote{quoteAt("10:00:05", "2500", "500")})

	executionContext, stopFollowing := context.WithCancel(context.Background())
	stopFollowing()

	proxy := marketdata.NewTwseRealtimeLiveMarketDataProxy(
		source.server.URL, twseTaiwanStockMarket(), time.Millisecond,
		2*time.Second, marketdata.NewRequestPacer(60))

	liveKCandles, followError := proxy.FollowKCandles(
		executionContext, vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}))

	assert.Error(t, followError)
	assert.Nil(t, liveKCandles)
}

// More candles than the feed will hold, and nobody reading them. Ending the follow
// has to release the send that is waiting for room — otherwise the goroutine behind
// every abandoned follow stays parked on a channel nobody will ever read, and the
// system leaks one per symbol per session.
func TestEndingAFollowReleasesASendWaitingForRoom(t *testing.T) {
	crowdedSymbols := make([]string, 0, 20)
	openingQuotes := make([]twseQuote, 0, 20)
	tradingQuotes := make([]twseQuote, 0, 20)
	for symbolIndex := range 20 {
		symbol := fmt.Sprintf("10%02d", symbolIndex)
		crowdedSymbols = append(crowdedSymbols, symbol)
		openingQuotes = append(openingQuotes, twseQuote{
			Symbol: symbol, LatestPrice: "100", CumulativeVolume: "10",
			LatestTradeTime: "10:00:05", TradingDate: "20260922",
		})
		tradingQuotes = append(tradingQuotes, twseQuote{
			Symbol: symbol, LatestPrice: "101", CumulativeVolume: "20",
			LatestTradeTime: "10:00:45", TradingDate: "20260922",
		})
	}

	source := newTwseSourceUnderTest(t, openingQuotes, tradingQuotes)

	liveKCandles, stopFollowing := followTwse(t, source, crowdedSymbols...)

	// Give the feed time to fill and block, then end the follow without reading a
	// single candle.
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

// Between two rounds of asking, the follow is parked waiting for the next one. Ending
// it there has to end it now rather than at the next round — a source asked once a
// minute would otherwise keep a cancelled follow alive for most of a minute, and
// every roster rebuild would leave one behind.
func TestEndingAFollowWaitingForItsNextRoundEndsItAtOnce(t *testing.T) {
	source := newTwseSourceUnderTest(t, []twseQuote{quoteAt("10:00:05", "2500", "500")})

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	// An interval no test would outwait, so the follow is certainly parked between
	// rounds rather than in the middle of one.
	proxy := marketdata.NewTwseRealtimeLiveMarketDataProxy(
		source.server.URL, twseTaiwanStockMarket(), time.Hour,
		2*time.Second, marketdata.NewRequestPacer(0))

	liveKCandles, followError := proxy.FollowKCandles(
		executionContext, vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}))
	require.NoError(t, followError)

	stopFollowing()

	require.Eventually(t, func() bool {
		select {
		case _, isOpen := <-liveKCandles:
			return !isOpen
		default:
			return false
		}
	}, 3*time.Second, 10*time.Millisecond)
}

// An address that is not an address is refused before anything is asked of the
// network. It can only come from a misconfiguration, and saying so at the first
// attempt is the difference between one clear refusal and a market that is quietly
// never followed.
func TestAnAddressThatCannotBeReachedIsRefusedAtOnce(t *testing.T) {
	proxy := marketdata.NewTwseRealtimeLiveMarketDataProxy(
		"://not-an-address", twseTaiwanStockMarket(), time.Millisecond,
		2*time.Second, marketdata.NewRequestPacer(0))

	liveKCandles, followError := proxy.FollowKCandles(
		context.Background(),
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}))

	assert.Error(t, followError)
	assert.Nil(t, liveKCandles)
}
