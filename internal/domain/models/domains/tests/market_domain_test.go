package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taipeiLocation is a fixed offset so tests don't depend on the machine's timezone database.
var taipeiLocation = time.FixedZone("Asia/Taipei", 8*60*60)

// taiwanStockRules: 09:00–13:30 Taipei, Monday–Friday, rostered, five symbols per channel.
func taiwanStockRules() vo.MarketRulesVo {
	return vo.MarketRulesVo{
		TradingSession: vo.TradingSessionVo{
			Location:   taipeiLocation,
			DailyStart: 9 * time.Hour,
			DailyEnd:   13*time.Hour + 30*time.Minute,
			Weekdays: []time.Weekday{
				time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
			},
		},
		FollowsFixedRoster:         true,
		SimultaneousChannelCeiling: 1,
		SymbolsPerLiveChannel:      5,
	}
}

func marketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto:      {},
		vo.MarketTaiwanStock: taiwanStockRules(),
	})
}

func taiwanStockMarket() domains.MarketDomain {
	return marketCatalog().MarketOf(string(vo.MarketTaiwanStock))
}

func cryptoMarket() domains.MarketDomain {
	return marketCatalog().MarketOf(string(vo.MarketCrypto))
}

// windowBetween takes Taipei-time moments so tables read in the requirements' clock.
func windowBetween(t *testing.T, market vo.MarketVo, startTime string, endTime string) vo.KCandleFetchWindowVo {
	t.Helper()

	return vo.NewKCandleFetchWindowVo(
		"2330", market, mustParseTime(t, startTime), mustParseTime(t, endTime))
}

func TestMarketOfReadsWhatWasStored(t *testing.T) {
	testCases := []struct {
		name           string
		storedMarket   string
		expectedMarket vo.MarketVo
	}{
		{name: "taiwan stock", storedMarket: "taiwanStock", expectedMarket: vo.MarketTaiwanStock},
		{name: "crypto", storedMarket: "crypto", expectedMarket: vo.MarketCrypto},
		// Rows predating the market column read as crypto, the only market at the time.
		{name: "a row that names no market", storedMarket: "", expectedMarket: vo.MarketCrypto},
		{name: "a market nobody offers", storedMarket: "nasdaq", expectedMarket: vo.MarketCrypto},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			marketDomain := marketCatalog().MarketOf(testCase.storedMarket)

			assert.Equal(t, testCase.expectedMarket, marketDomain.Value())
		})
	}
}

func TestIsRecognisedSeparatesANamedMarketFromAnAbsentOne(t *testing.T) {
	catalog := marketCatalog()

	assert.True(t, catalog.IsRecognised("taiwanStock"))
	assert.True(t, catalog.IsRecognised("crypto"))
	// Unknown market names are refused, even though an empty stored market is forgiven.
	assert.False(t, catalog.IsRecognised("nasdaq"))
	assert.False(t, catalog.IsRecognised(""))
	assert.Equal(t, []string{"crypto", "taiwanStock"}, catalog.RecognisedMarkets())
}

func TestTaiwanStockIsOpenOnlyDuringItsTradingSession(t *testing.T) {
	testCases := []struct {
		name           string
		moment         string
		expectedIsOpen bool
	}{
		{name: "mid session on a weekday", moment: "2026-09-08T10:07:00+08:00", expectedIsOpen: true},
		{name: "the moment it opens", moment: "2026-09-08T09:00:00+08:00", expectedIsOpen: true},
		{name: "the last minute before it closes", moment: "2026-09-08T13:29:00+08:00", expectedIsOpen: true},
		{name: "the moment it closes", moment: "2026-09-08T13:30:00+08:00", expectedIsOpen: false},
		{name: "just before it opens", moment: "2026-09-08T08:59:00+08:00", expectedIsOpen: false},
		{name: "the evening", moment: "2026-09-08T21:00:00+08:00", expectedIsOpen: false},
		{name: "saturday", moment: "2026-09-12T10:07:00+08:00", expectedIsOpen: false},
		{name: "sunday", moment: "2026-09-13T10:07:00+08:00", expectedIsOpen: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			isOpen := taiwanStockMarket().IsOpen(mustParseTime(t, testCase.moment))

			assert.Equal(t, testCase.expectedIsOpen, isOpen)
		})
	}
}

func TestCryptoIsOpenWheneverItIsAsked(t *testing.T) {
	testCases := []struct {
		name   string
		moment string
	}{
		{name: "the middle of a weekday", moment: "2026-09-08T10:07:00+08:00"},
		{name: "the middle of the night", moment: "2026-09-08T03:00:00+08:00"},
		{name: "sunday", moment: "2026-09-13T21:00:00+08:00"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.True(t, cryptoMarket().IsOpen(mustParseTime(t, testCase.moment)))
		})
	}
}

func TestClampToTradingSessionKeepsOnlyWhatCouldHoldCandles(t *testing.T) {
	testCases := []struct {
		name              string
		windowStart       string
		windowEnd         string
		expectedIsEmpty   bool
		expectedStartTime string
		expectedEndTime   string
	}{
		{
			name:              "a round in the middle of the session",
			windowStart:       "2026-09-08T09:40:00+08:00",
			windowEnd:         "2026-09-08T10:00:00+08:00",
			expectedStartTime: "2026-09-08T09:40:00+08:00",
			expectedEndTime:   "2026-09-08T10:00:00+08:00",
		},
		{
			// 13:29 is the candle that finished exactly at 13:30.
			name:              "the round just after the close still reaches the day's last candle",
			windowStart:       "2026-09-08T13:05:00+08:00",
			windowEnd:         "2026-09-08T13:29:00+08:00",
			expectedStartTime: "2026-09-08T13:05:00+08:00",
			expectedEndTime:   "2026-09-08T13:29:00+08:00",
		},
		{
			// Cut back to the last candle the session can hold, not the closing bell.
			name:              "a window reaching past the close stops at the last candle",
			windowStart:       "2026-09-08T13:20:00+08:00",
			windowEnd:         "2026-09-08T14:00:00+08:00",
			expectedStartTime: "2026-09-08T13:20:00+08:00",
			expectedEndTime:   "2026-09-08T13:29:00+08:00",
		},
		{
			name:            "an evening round covers nothing",
			windowStart:     "2026-09-08T20:40:00+08:00",
			windowEnd:       "2026-09-08T21:00:00+08:00",
			expectedIsEmpty: true,
		},
		{
			name:            "a sunday round covers nothing",
			windowStart:     "2026-09-13T09:40:00+08:00",
			windowEnd:       "2026-09-13T10:00:00+08:00",
			expectedIsEmpty: true,
		},
		{
			name:              "a saturday backfill reaches back into friday's session",
			windowStart:       "2026-09-11T09:00:00+08:00",
			windowEnd:         "2026-09-12T08:55:00+08:00",
			expectedStartTime: "2026-09-11T09:00:00+08:00",
			expectedEndTime:   "2026-09-11T13:29:00+08:00",
		},
		{
			// The closed gap between Friday and Monday isn't a gap, so only this morning is filled.
			name:              "a monday backfill after a complete friday fills only this morning",
			windowStart:       "2026-09-11T13:30:00+08:00",
			windowEnd:         "2026-09-14T09:25:00+08:00",
			expectedStartTime: "2026-09-14T09:00:00+08:00",
			expectedEndTime:   "2026-09-14T09:25:00+08:00",
		},
		{
			name:              "a mid-session backfill starts where the stored candles ended",
			windowStart:       "2026-09-08T09:35:00+08:00",
			windowEnd:         "2026-09-08T10:55:00+08:00",
			expectedStartTime: "2026-09-08T09:35:00+08:00",
			expectedEndTime:   "2026-09-08T10:55:00+08:00",
		},
		{
			name:            "a window falling entirely between two sessions covers nothing",
			windowStart:     "2026-09-11T14:00:00+08:00",
			windowEnd:       "2026-09-13T23:00:00+08:00",
			expectedIsEmpty: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clampedWindow := taiwanStockMarket().ClampToTradingSession(
				windowBetween(t, vo.MarketTaiwanStock, testCase.windowStart, testCase.windowEnd))

			if testCase.expectedIsEmpty {
				assert.True(t, clampedWindow.IsEmpty())

				return
			}

			assert.False(t, clampedWindow.IsEmpty())
			assert.Equal(t, mustParseTime(t, testCase.expectedStartTime).UTC(), clampedWindow.StartTime)
			assert.Equal(t, mustParseTime(t, testCase.expectedEndTime).UTC(), clampedWindow.EndTime)
			assert.Equal(t, vo.MarketTaiwanStock, clampedWindow.Market)
			assert.Equal(t, "2330", clampedWindow.Symbol)
		})
	}
}

func TestClampToTradingSessionLeavesARoundTheClockMarketAlone(t *testing.T) {
	// A never-closing market has nothing to narrow.
	window := windowBetween(
		t, vo.MarketCrypto, "2026-09-11T14:00:00+08:00", "2026-09-13T23:00:00+08:00")

	clampedWindow := cryptoMarket().ClampToTradingSession(window)

	assert.Equal(t, window, clampedWindow)
	assert.False(t, clampedWindow.IsEmpty())
}

func TestClampToTradingSessionLeavesAnAlreadyEmptyWindowEmpty(t *testing.T) {
	// An empty window must stay empty after narrowing.
	emptyWindow := windowBetween(
		t, vo.MarketTaiwanStock, "2026-09-08T10:05:00+08:00", "2026-09-08T10:00:00+08:00")

	assert.True(t, taiwanStockMarket().ClampToTradingSession(emptyWindow).IsEmpty())
}

// The follow ceiling is derived from the two plan numbers (channels × symbols per channel) so it can't contradict them.
func TestSimultaneousFollowCeilingIsTheTwoPlanNumbersMultiplied(t *testing.T) {
	testCases := []struct {
		name            string
		rules           vo.MarketRulesVo
		expectedCeiling int
	}{
		{
			name:            "一條通道乘上每條五檔是五檔",
			rules:           vo.MarketRulesVo{SimultaneousChannelCeiling: 1, SymbolsPerLiveChannel: 5},
			expectedCeiling: 5,
		},
		{
			name:            "兩條通道各跟三檔是六檔",
			rules:           vo.MarketRulesVo{SimultaneousChannelCeiling: 2, SymbolsPerLiveChannel: 3},
			expectedCeiling: 6,
		},
		{
			name:            "一條通道只跟一檔是一檔",
			rules:           vo.MarketRulesVo{SimultaneousChannelCeiling: 1, SymbolsPerLiveChannel: 1},
			expectedCeiling: 1,
		},
		{
			name:            "沒說一條跟幾檔就是一檔",
			rules:           vo.MarketRulesVo{SimultaneousChannelCeiling: 3},
			expectedCeiling: 3,
		},
		{
			name:            "不限通道數就不設上限",
			rules:           vo.MarketRulesVo{},
			expectedCeiling: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			marketDomain := domains.NewMarketCatalogDomain(
				map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: testCase.rules}).
				MarketOf(string(vo.MarketCrypto))

			assert.Equal(t, testCase.expectedCeiling, marketDomain.SimultaneousFollowCeiling())
		})
	}
}

func TestSimultaneousFollowCeilingIsTheMarketsOwn(t *testing.T) {
	assert.Equal(t, 5, taiwanStockMarket().SimultaneousFollowCeiling())
	// No ceiling means follows are viewer-driven rather than rostered.
	assert.Equal(t, 0, cryptoMarket().SimultaneousFollowCeiling())
}

func TestSymbolsPerLiveChannelIsNeverFewerThanOne(t *testing.T) {
	assert.Equal(t, 5, taiwanStockMarket().SymbolsPerLiveChannel())
	assert.Equal(t, 1, cryptoMarket().SymbolsPerLiveChannel())
}

func TestTradingDateOfIsTheMarketsOwnDay(t *testing.T) {
	// Both are one Taiwan trading day despite differing UTC dates, so a presumed-closed market lasts until its own tomorrow.
	morning := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-08T10:00:00+08:00"))
	lateEvening := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-08T23:00:00+08:00"))
	nextMorning := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-09T10:00:00+08:00"))

	assert.Equal(t, morning, lateEvening)
	assert.NotEqual(t, morning, nextMorning)
	assert.Equal(t, mustParseTime(t, "2026-09-08T00:00:00+08:00").UTC(), morning)
}

func TestTradingDateOfARoundTheClockMarketIsItsUniversalDay(t *testing.T) {
	// A zoneless market uses the UTC day; it is never presumed closed, so this only needs consistency.
	tradingDate := cryptoMarket().TradingDateOf(mustParseTime(t, "2026-09-08T10:00:00+08:00"))

	assert.Equal(t, mustParseTime(t, "2026-09-08T00:00:00Z"), tradingDate)
}

func TestACatalogAlwaysRecognisesTheMarketItFallsBackTo(t *testing.T) {
	// The catalog always restores the crypto fallback so an unnamed stored market resolves to real rules.
	catalogWithoutCrypto := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: taiwanStockRules(),
	})

	fallbackMarket := catalogWithoutCrypto.MarketOf("")

	assert.Equal(t, vo.MarketCrypto, fallbackMarket.Value())
	assert.True(t, fallbackMarket.IsOpen(mustParseTime(t, "2026-09-13T21:00:00+08:00")))
	assert.True(t, catalogWithoutCrypto.IsRecognised("crypto"))
}

func TestASessionKeepsItsClockReadingOnADayThatLosesAnHour(t *testing.T) {
	// A DST zone still opens at 09:00 local on the spring-forward day (London, 2026-03-29, 23 hours long), not midnight plus nine hours.
	london, loadError := time.LoadLocation("Europe/London")
	require.NoError(t, loadError)
	marketDomain := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   london,
				DailyStart: 9 * time.Hour,
				DailyEnd:   17 * time.Hour,
				Weekdays: []time.Weekday{
					time.Sunday, time.Monday, time.Tuesday, time.Wednesday,
					time.Thursday, time.Friday, time.Saturday,
				},
			},
		},
	}).MarketOf(string(vo.MarketTaiwanStock))

	clamped := marketDomain.ClampToTradingSession(vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock,
		time.Date(2026, 3, 29, 0, 0, 0, 0, london).UTC(),
		time.Date(2026, 3, 30, 0, 0, 0, 0, london).UTC(),
	))

	require.False(t, clamped.IsEmpty())
	assert.Equal(t,
		time.Date(2026, 3, 29, 9, 0, 0, 0, london).UTC(), clamped.StartTime,
		"開盤是「早上九點」，不是「午夜之後九小時」")
}

// 數格子而非以交易時間除以刻度長度；起訖兩端都算，所以整整一小時的盤中有 61 個一分鐘格子。
func TestTradingBucketCountCountsOnlyBucketsThatHoldTrading(t *testing.T) {
	testCases := []struct {
		name                string
		startTime           string
		endTime             string
		bucketDuration      time.Duration
		expectedBucketCount int
	}{
		{
			name:      "a whole session at one minute",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 270,
		},
		{
			name:      "a whole day around one session at one minute",
			startTime: "2026-09-06T13:30:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 270,
		},
		{
			name:      "wholly inside a session at one minute",
			startTime: "2026-09-07T11:00:00+08:00", endTime: "2026-09-07T12:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 61,
		},
		{
			name:      "across one close at one minute",
			startTime: "2026-09-07T13:00:00+08:00", endTime: "2026-09-08T10:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 91,
		},
		{
			name:      "across a weekend at one minute",
			startTime: "2026-09-11T12:00:00+08:00", endTime: "2026-09-14T10:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 151,
		},
		{
			name:      "wholly after the close",
			startTime: "2026-09-07T14:00:00+08:00", endTime: "2026-09-07T16:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 0,
		},
		{
			name:      "a whole Saturday",
			startTime: "2026-09-12T00:00:00+08:00", endTime: "2026-09-13T00:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 0,
		},
		{
			// 兩端都算，所以開盤那一刻的那一根落在裡面。
			name:      "the boundary: it ends exactly at the opening bell",
			startTime: "2026-09-07T08:00:00+08:00", endTime: "2026-09-07T09:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 1,
		},
		{
			// 當日最後一根開在收盤前一分鐘。
			name:      "the boundary: it begins exactly at the closing bell",
			startTime: "2026-09-07T13:30:00+08:00", endTime: "2026-09-07T15:00:00+08:00",
			bucketDuration: time.Minute, expectedBucketCount: 0,
		},
		{
			// 四個半小時除以一小時是四，但它碰到 UTC 01–05 五個整點格子。
			name:      "a whole session at one hour is five buckets, not four",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			bucketDuration: time.Hour, expectedBucketCount: 5,
		},
		{
			name:      "a whole session at four hours is two buckets, not one",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			bucketDuration: 4 * time.Hour, expectedBucketCount: 2,
		},
		{
			name:      "a whole session at one day is one bucket",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			bucketDuration: 24 * time.Hour, expectedBucketCount: 1,
		},
		{
			// 除法會說 22.5 小時 ÷ 24 小時不到一格，使「一次最多一千根」失真。
			name:      "five sessions at one day are five buckets, not a fifth of one",
			startTime: "2026-09-07T00:00:00+08:00", endTime: "2026-09-12T00:00:00+08:00",
			bucketDuration: 24 * time.Hour, expectedBucketCount: 5,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bucketCount := taiwanStockMarket().TradingBucketCountBetween(
				mustParseTime(t, testCase.startTime),
				mustParseTime(t, testCase.endTime),
				testCase.bucketDuration)

			assert.Equal(t, testCase.expectedBucketCount, bucketCount)
		})
	}
}

func TestTradingBucketCountIsTheWholeStretchForAMarketThatNeverCloses(t *testing.T) {
	testCases := []struct {
		name                string
		startTime           string
		endTime             string
		bucketDuration      time.Duration
		expectedBucketCount int
	}{
		{
			name:      "a whole day at one hour",
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-08T00:00:00Z",
			bucketDuration: time.Hour, expectedBucketCount: 24,
		},
		{
			name:      "a whole day at one minute",
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-08T00:00:00Z",
			bucketDuration: time.Minute, expectedBucketCount: 1440,
		},
		{
			name:      "a Saturday counts like any other day",
			startTime: "2026-09-12T00:00:00Z", endTime: "2026-09-13T00:00:00Z",
			bucketDuration: time.Hour, expectedBucketCount: 24,
		},
		{
			name:      "a year at one day",
			startTime: "2026-01-01T00:00:00Z", endTime: "2027-01-01T00:00:00Z",
			bucketDuration: 24 * time.Hour, expectedBucketCount: 365,
		},
		{
			name:      "a stretch shorter than one bucket",
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-07T00:03:00Z",
			bucketDuration: 5 * time.Minute, expectedBucketCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bucketCount := cryptoMarket().TradingBucketCountBetween(
				mustParseTime(t, testCase.startTime),
				mustParseTime(t, testCase.endTime),
				testCase.bucketDuration)

			assert.Equal(t, testCase.expectedBucketCount, bucketCount)
		})
	}
}

// 有沒有交易要直接問，不能拿格數當答案：全天候市場的三十秒裝不滿一個一分鐘格子，格數會取整為零。
func TestHoldsTradingAnswersTheQuestionItself(t *testing.T) {
	testCases := []struct {
		name                 string
		market               domains.MarketDomain
		startTime            string
		endTime              string
		expectedHoldsTrading bool
	}{
		{
			name:      "全天候市場的三十秒：有交易，即使裝不滿一格",
			market:    cryptoMarket(),
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-07T00:00:30Z",
			expectedHoldsTrading: true,
		},
		{
			name:      "全天候市場的起訖相同：沒有任何一刻在裡面",
			market:    cryptoMarket(),
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-07T00:00:00Z",
			expectedHoldsTrading: false,
		},
		{
			name:      "台股盤中的一小時：有交易",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T11:00:00+08:00", endTime: "2026-09-07T12:00:00+08:00",
			expectedHoldsTrading: true,
		},
		{
			name:      "台股盤中的三十秒：有交易，即使裝不滿一格",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T11:00:00+08:00", endTime: "2026-09-07T11:00:30+08:00",
			expectedHoldsTrading: true,
		},
		{
			name:      "台股整個週六：沒有交易",
			market:    taiwanStockMarket(),
			startTime: "2026-09-12T00:00:00+08:00", endTime: "2026-09-13T00:00:00+08:00",
			expectedHoldsTrading: false,
		},
		{
			name:      "台股收盤之後：沒有交易",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T14:00:00+08:00", endTime: "2026-09-07T16:00:00+08:00",
			expectedHoldsTrading: false,
		},
		{
			name:      "台股從收盤鐘聲起算：沒有交易——當日最後一根開在收盤前一分鐘",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T13:30:00+08:00", endTime: "2026-09-07T15:00:00+08:00",
			expectedHoldsTrading: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			holdsTrading := testCase.market.HoldsTrading(
				mustParseTime(t, testCase.startTime), mustParseTime(t, testCase.endTime))

			assert.Equal(t, testCase.expectedHoldsTrading, holdsTrading)
		})
	}
}

// 數一個世紀（一分鐘刻度九百萬格）必須用算術而非逐格走訪，否則會在拒絕之前吃掉近 1 GB；這裡只釘住答得出來，不量記憶體。
func TestCountingACenturyIsStillAnswered(t *testing.T) {
	bucketCount := taiwanStockMarket().TradingBucketCountBetween(
		mustParseTime(t, "1926-01-01T00:00:00Z"),
		mustParseTime(t, "2026-01-01T00:00:00Z"),
		time.Minute)

	assert.Positive(t, bucketCount)
}

// Whether a market is rostered is its own setting, no longer inferred from its subscription cap.
func TestAMarketSaysWhetherItIsFollowedFromARoster(t *testing.T) {
	testCases := []struct {
		name               string
		rules              vo.MarketRulesVo
		followsFixedRoster bool
		hasFollowCeiling   bool
	}{
		{
			name:               "rostered and capped",
			rules:              vo.MarketRulesVo{FollowsFixedRoster: true, SimultaneousChannelCeiling: 1},
			followsFixedRoster: true,
			hasFollowCeiling:   true,
		},
		{
			name:               "rostered with no cap at all",
			rules:              vo.MarketRulesVo{FollowsFixedRoster: true},
			followsFixedRoster: true,
			hasFollowCeiling:   false,
		},
		{
			name:               "followed by whoever looks",
			rules:              vo.MarketRulesVo{},
			followsFixedRoster: false,
			hasFollowCeiling:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			marketDomain := domains.NewMarketCatalogDomain(
				map[vo.MarketVo]vo.MarketRulesVo{vo.MarketTaiwanStock: testCase.rules}).
				MarketOf(string(vo.MarketTaiwanStock))

			assert.Equal(t, testCase.followsFixedRoster, marketDomain.FollowsFixedRoster())
			assert.Equal(t, testCase.hasFollowCeiling, marketDomain.HasFollowCeiling())
		})
	}
}
