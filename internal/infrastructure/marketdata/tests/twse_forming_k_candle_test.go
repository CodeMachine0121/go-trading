package marketdata_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The exchange is asked rather than subscribed to, so the only way to exercise the
// folding is through the proxy that does the asking — which is also the only way the
// requirements describe it.

var twseTaipeiLocation = time.FixedZone("Asia/Taipei", 8*60*60)

func twseTaiwanStockMarket() domains.MarketDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   twseTaipeiLocation,
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			FollowsFixedRoster:    true,
			SymbolsPerLiveChannel: 25,
		},
	}).MarketOf(string(vo.MarketTaiwanStock))
}

// twseQuote is one entry of the answer, written the way the source writes it: every
// figure as text, volume in lots, the running total for the whole day.
type twseQuote struct {
	Symbol           string `json:"c"`
	LatestPrice      string `json:"z"`
	CumulativeVolume string `json:"v"`
	LatestTradeTime  string `json:"t"`
	TradingDate      string `json:"d"`
}

func quoteAt(tradeTime string, price string, cumulativeLots string) twseQuote {
	return twseQuote{
		Symbol: "2330", LatestPrice: price, CumulativeVolume: cumulativeLots,
		LatestTradeTime: tradeTime, TradingDate: "20260922",
	}
}

// twseSourceUnderTest answers each request with the next answer the test handed it,
// repeating the last one once it runs out — which is exactly what the real source
// does between trades.
type twseSourceUnderTest struct {
	server         *httptest.Server
	answers        [][]twseQuote
	returnCode     string
	statusCode     int
	requestedExChs chan string
	answeredCount  int
	// rawBody, when set, is written instead of a well-formed answer — which is how a
	// source that cannot be read is told apart from one with nothing to report.
	rawBody string
	// failsAfter is how many answers the source gives before it stops answering at
	// all, so that a feed breaking mid-follow can be told from one that never opened.
	failsAfter int
}

func newTwseSourceUnderTest(t *testing.T, answers ...[]twseQuote) *twseSourceUnderTest {
	t.Helper()

	source := &twseSourceUnderTest{
		answers:        answers,
		returnCode:     "0000",
		statusCode:     http.StatusOK,
		requestedExChs: make(chan string, 32),
	}

	source.server = httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter, request *http.Request,
	) {
		select {
		case source.requestedExChs <- request.URL.Query().Get("ex_ch"):
		default:
		}

		if source.failsAfter > 0 && source.answeredCount >= source.failsAfter {
			responseWriter.WriteHeader(http.StatusInternalServerError)

			return
		}

		if source.statusCode != http.StatusOK {
			responseWriter.WriteHeader(source.statusCode)

			return
		}

		if source.rawBody != "" {
			source.answeredCount++
			_, _ = responseWriter.Write([]byte(source.rawBody))

			return
		}

		answer := []twseQuote{}
		if len(source.answers) > 0 {
			index := min(source.answeredCount, len(source.answers)-1)
			answer = source.answers[index]
		}
		source.answeredCount++

		responseWriter.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{
			"msgArray": answer, "rtcode": source.returnCode, "rtmessage": "OK",
		})
	}))
	t.Cleanup(source.server.Close)

	return source
}

func followTwse(
	t *testing.T, source *twseSourceUnderTest, symbols ...string,
) (<-chan vo.LiveKCandleVo, context.CancelFunc) {
	t.Helper()

	executionContext, stopFollowing := context.WithCancel(context.Background())
	t.Cleanup(stopFollowing)

	proxy := marketdata.NewTwseRealtimeLiveMarketDataProxy(
		source.server.URL, twseTaiwanStockMarket(), time.Millisecond,
		2*time.Second)

	liveKCandles, followError := proxy.FollowKCandles(
		executionContext, vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, symbols))
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

// The running total is the day's, so a minute's volume is what it grew by. Five
// hundred lots when the minute opened and five hundred and thirty now is thirty lots
// traded — thirty thousand shares.
func TestAMinuteVolumeIsWhatTheDayTotalGrewByWithinIt(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:00:45", "2510", "530")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	liveKCandle := nextKCandle(t, liveKCandles)
	assert.Equal(t, "30000", liveKCandle.Volume.String())
	assert.False(t, liveKCandle.Closed)
}

// Lots are what this venue counts in; shares are what the system speaks. A thousand
// lots is a million shares, and getting this wrong is silent — both are believable
// volumes.
func TestVolumeIsCarriedInSharesRatherThanLots(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "0")},
		[]twseQuote{quoteAt("10:00:45", "2500", "1000")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	assert.Equal(t, "1000000", nextKCandle(t, liveKCandles).Volume.String())
}

// A running total that has not moved is a minute nothing traded in. There is no
// candle for it, which is the same thing the history source says by leaving it out.
func TestAMinuteNothingTradedInIsNotReported(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:01:10", "2520", "560")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	// The only candle that can arrive is the next minute's, because the first minute
	// never grew. A finished candle for it would have come first if it existed.
	liveKCandle := nextKCandle(t, liveKCandles)
	assert.Equal(t, "60000", liveKCandle.Volume.String())
	assert.False(t, liveKCandle.Closed)
}

// A quote landing in a later minute is what proves the earlier one finished, and the
// finished one is reported first so that no chart ever goes backwards.
//
// The minute measured here is the second one, because it is the first the follow saw
// whole. See the test below for why the one it joined in is never finished.
func TestAQuoteInALaterMinuteFinishesTheEarlierOneFirst(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:01:10", "2520", "600")},
		[]twseQuote{quoteAt("10:01:50", "2530", "660")},
		[]twseQuote{quoteAt("10:02:10", "2540", "700")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	finished := firstFinishedKCandle(t, liveKCandles)
	assert.Equal(t,
		time.Date(2026, 9, 22, 10, 1, 0, 0, twseTaipeiLocation).UTC(),
		finished.OpenTime.UTC())
	assert.Equal(t, "160000", finished.Volume.String())
	assert.Equal(t, "2520", finished.Open.String())
	assert.Equal(t, "2530", finished.Close.String())
}

// The minute a follow opens in is never reported finished, however much trades in it
// afterwards.
//
// A finished candle is the one that gets stored, and stored it is permanent: the live
// path saves by overwriting, while the scheduled round only fills in what is missing.
// A minute counted from part way through — which is every minute a follow opens in —
// would therefore become the lasting answer for that minute, with the round that
// could have supplied it whole locked out for good.
//
// It matters far beyond start-up: a follow opens on every roster rebuild, which is
// every watchlist change and every recovery from a stall.
func TestTheMinuteAFollowJoinsIsNeverReportedFinished(t *testing.T) {
	joinedMinute := time.Date(2026, 9, 22, 10, 0, 0, 0, twseTaipeiLocation).UTC()

	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:00:45", "2510", "560")},
		[]twseQuote{quoteAt("10:01:10", "2520", "600")},
		[]twseQuote{quoteAt("10:01:50", "2530", "640")},
		[]twseQuote{quoteAt("10:02:10", "2540", "700")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	// Read until a later minute has been finished, which proves the joined one was
	// passed over rather than merely not reported yet.
	for range 12 {
		liveKCandle := nextKCandle(t, liveKCandles)
		assert.False(t, liveKCandle.Closed && liveKCandle.OpenTime.Equal(joinedMinute),
			"the minute the follow joined must never be stored")

		if liveKCandle.Closed {
			return
		}
	}

	t.Fatal("no later minute was ever finished")
}

// A running total that goes backwards is a new day or a source that restarted its
// count. Either way the figure in hand is the truth from here, and a negative volume
// is a number nothing downstream is built to disbelieve.
func TestARunningTotalGoingBackwardsNeverProducesANegativeVolume(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:00:20", "2505", "530")},
		[]twseQuote{quoteAt("10:00:40", "2495", "20")},
		[]twseQuote{quoteAt("10:00:55", "2495", "35")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	for range 2 {
		liveKCandle := nextKCandle(t, liveKCandles)
		assert.False(t, liveKCandle.Volume.IsNegative(),
			"a restarted count must never read as a negative volume")
	}
}

// A minute's shape is the shape of the points inside it — where it opened, how far it
// reached either way, and where it stands. Taking this source's own high and low
// would take the whole day's.
func TestAMinuteTakesItsShapeFromThePointsInsideIt(t *testing.T) {
	// The shape is measured on the second minute: the one the follow joined is never
	// finished, and its opening price is whatever happened to be quoted when we
	// arrived rather than the minute's own open.
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:01:02", "2505", "510")},
		[]twseQuote{quoteAt("10:01:20", "2530", "515")},
		[]twseQuote{quoteAt("10:01:40", "2480", "520")},
		[]twseQuote{quoteAt("10:01:55", "2495", "530")},
		[]twseQuote{quoteAt("10:02:10", "2495", "540")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	finished := firstFinishedKCandle(t, liveKCandles)

	assert.Equal(t, "2505", finished.Open.String())
	assert.Equal(t, "2530", finished.High.String())
	assert.Equal(t, "2480", finished.Low.String())
	assert.Equal(t, "2495", finished.Close.String())
}

// firstFinishedKCandle reads until a minute is reported finished, which is the only
// kind that gets stored and therefore the only kind worth measuring figures on.
func firstFinishedKCandle(
	t *testing.T, liveKCandles <-chan vo.LiveKCandleVo,
) vo.LiveKCandleVo {
	t.Helper()

	for range 12 {
		liveKCandle := nextKCandle(t, liveKCandles)
		if liveKCandle.Closed {
			return liveKCandle
		}
	}

	t.Fatal("no minute was ever reported finished")

	return vo.LiveKCandleVo{}
}

// The minute a candle belongs to is decided by when the trade happened, never by when
// the answer arrived. A source a few seconds behind would otherwise file a trade in
// the minute after the one it happened in, and the stored candle would be wrong
// rather than merely late.
func TestACandleIsFiledByTradeTimeRatherThanArrivalTime(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:00:59", "2510", "530")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	liveKCandle := nextKCandle(t, liveKCandles)
	assert.Equal(t,
		time.Date(2026, 9, 22, 10, 0, 0, 0, twseTaipeiLocation).UTC(),
		liveKCandle.OpenTime.UTC())
}

// A quote older than the minute being built says nothing new — the source restating
// something already superseded, or answers arriving out of order. Folding it in would
// reopen a minute that has already been reported finished.
func TestAQuoteFromAnAlreadyFinishedMinuteIsIgnored(t *testing.T) {
	source := newTwseSourceUnderTest(t,
		[]twseQuote{quoteAt("10:00:05", "2500", "500")},
		[]twseQuote{quoteAt("10:01:10", "2520", "560")},
		[]twseQuote{quoteAt("10:00:50", "9999", "999")},
		[]twseQuote{quoteAt("10:01:40", "2530", "590")},
	)

	liveKCandles, _ := followTwse(t, source, "2330")

	// Two candles can arrive: the minute the second quote opened, and that same
	// minute again once the fourth folds in. The stale third contributes to neither —
	// neither as a price nor as volume.
	for range 2 {
		liveKCandle := nextKCandle(t, liveKCandles)
		assert.NotEqual(t, "9999", liveKCandle.High.String(),
			"a superseded quote must not reopen a finished minute")
	}
}

// Figures this source cannot state are not the same as figures it states badly. A
// volume that is not a number at all is a source that cannot be read, and it must not
// become a candle.
func TestAQuoteWithUnreadableFiguresNeverBecomesACandle(t *testing.T) {
	testCases := []struct {
		name  string
		quote twseQuote
	}{
		{
			name: "a volume that is not a number",
			quote: twseQuote{Symbol: "2330", LatestPrice: "2500",
				CumulativeVolume: "lots and lots", LatestTradeTime: "10:00:05",
				TradingDate: "20260922"},
		},
		{
			name: "a trade time that is not a time",
			quote: twseQuote{Symbol: "2330", LatestPrice: "2500",
				CumulativeVolume: "500", LatestTradeTime: "half past ten",
				TradingDate: "20260922"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := newTwseSourceUnderTest(t,
				[]twseQuote{testCase.quote},
				[]twseQuote{quoteAt("10:00:45", "2510", "530")},
			)

			liveKCandles, _ := followTwse(t, source, "2330")

			// Only the readable quote can establish anything, so the first candle can
			// only come from a later one — never from the unreadable figures.
			select {
			case liveKCandle := <-liveKCandles:
				assert.Equal(t, "2510", liveKCandle.Close.String())
			case <-time.After(time.Second):
			}
		})
	}
}
