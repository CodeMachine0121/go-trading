package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taipeiLocation is fixed rather than loaded so that these rules are checked against
// the offset the requirements name, on any machine, whatever time zone database it
// happens to ship with.
var taipeiLocation = time.FixedZone("Asia/Taipei", 8*60*60)

// taiwanStockRules are the rules the requirements name: 09:00 to 13:30 Taipei time,
// Monday to Friday, five symbols followed live at once.
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
		SimultaneousFollowCeiling: 5,
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

// windowBetween builds a fetch window from two Taipei-time moments, so the tables
// below read in the same clock the requirements are written in.
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
		// A row written before markets were ever recorded names nothing. Reading it as
		// the market this system had at the time is the only reading that keeps it
		// behaving as it did.
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
	// Naming a market that does not exist is a request to refuse, even though reading
	// a row that named nothing is forgiven.
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
			// A round during the session asks for the last few closed candles and gets
			// exactly those back.
			name:              "a round in the middle of the session",
			windowStart:       "2026-09-08T09:40:00+08:00",
			windowEnd:         "2026-09-08T10:00:00+08:00",
			expectedStartTime: "2026-09-08T09:40:00+08:00",
			expectedEndTime:   "2026-09-08T10:00:00+08:00",
		},
		{
			// The round just after the close still has that day's last candle to
			// collect — 13:29 is the one that finished exactly at 13:30.
			name:              "the round just after the close still reaches the day's last candle",
			windowStart:       "2026-09-08T13:05:00+08:00",
			windowEnd:         "2026-09-08T13:29:00+08:00",
			expectedStartTime: "2026-09-08T13:05:00+08:00",
			expectedEndTime:   "2026-09-08T13:29:00+08:00",
		},
		{
			// A window reaching past the close is cut back to the last candle the
			// session could hold, not to the closing bell itself.
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
			// Starting up on a Saturday morning: the day before is the only stretch in
			// reach that could hold anything.
			name:              "a saturday backfill reaches back into friday's session",
			windowStart:       "2026-09-11T09:00:00+08:00",
			windowEnd:         "2026-09-12T08:55:00+08:00",
			expectedStartTime: "2026-09-11T09:00:00+08:00",
			expectedEndTime:   "2026-09-11T13:29:00+08:00",
		},
		{
			// Starting up on Monday morning with Friday already complete: the gap in
			// between is not a gap, so only this morning is left to fill.
			name:              "a monday backfill after a complete friday fills only this morning",
			windowStart:       "2026-09-11T13:30:00+08:00",
			windowEnd:         "2026-09-14T09:25:00+08:00",
			expectedStartTime: "2026-09-14T09:00:00+08:00",
			expectedEndTime:   "2026-09-14T09:25:00+08:00",
		},
		{
			// Mid-session backfill: the start stays where the stored data ended rather
			// than jumping back to the opening bell.
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
	// A market that never closes has nothing to narrow: the night and the weekend are
	// as tradable as any other hour, so a backfill across them stays whole.
	window := windowBetween(
		t, vo.MarketCrypto, "2026-09-11T14:00:00+08:00", "2026-09-13T23:00:00+08:00")

	clampedWindow := cryptoMarket().ClampToTradingSession(window)

	assert.Equal(t, window, clampedWindow)
	assert.False(t, clampedWindow.IsEmpty())
}

func TestClampToTradingSessionLeavesAnAlreadyEmptyWindowEmpty(t *testing.T) {
	// A symbol already up to date produces a window that covers nothing. Narrowing it
	// must not turn it into something.
	emptyWindow := windowBetween(
		t, vo.MarketTaiwanStock, "2026-09-08T10:05:00+08:00", "2026-09-08T10:00:00+08:00")

	assert.True(t, taiwanStockMarket().ClampToTradingSession(emptyWindow).IsEmpty())
}

func TestSimultaneousFollowCeilingIsTheMarketsOwn(t *testing.T) {
	assert.Equal(t, 5, taiwanStockMarket().SimultaneousFollowCeiling())
	// No ceiling is how a market says its follows are driven by viewers rather than
	// by a roster.
	assert.Equal(t, 0, cryptoMarket().SimultaneousFollowCeiling())
}

func TestTradingDateOfIsTheMarketsOwnDay(t *testing.T) {
	// Both of these are the same Taiwan trading day, though they fall on different
	// days in universal time. Presuming a market closed must last until its own
	// tomorrow, not until midnight somewhere else.
	morning := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-08T10:00:00+08:00"))
	lateEvening := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-08T23:00:00+08:00"))
	nextMorning := taiwanStockMarket().TradingDateOf(mustParseTime(t, "2026-09-09T10:00:00+08:00"))

	assert.Equal(t, morning, lateEvening)
	assert.NotEqual(t, morning, nextMorning)
	assert.Equal(t, mustParseTime(t, "2026-09-08T00:00:00+08:00").UTC(), morning)
}

func TestTradingDateOfARoundTheClockMarketIsItsUniversalDay(t *testing.T) {
	// A market with no zone to say its hours in has no local calendar either, so its
	// day is the universal one. It is never presumed closed, so this answer only has
	// to be consistent — and it is.
	tradingDate := cryptoMarket().TradingDateOf(mustParseTime(t, "2026-09-08T10:00:00+08:00"))

	assert.Equal(t, mustParseTime(t, "2026-09-08T00:00:00Z"), tradingDate)
}

func TestACatalogAlwaysRecognisesTheMarketItFallsBackTo(t *testing.T) {
	// Reading a row that names no market must always land on rules that exist. If the
	// fallback could be left out, that reading would produce a market nobody could
	// answer questions about — so the catalog puts it back whatever it was handed.
	catalogWithoutCrypto := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: taiwanStockRules(),
	})

	fallbackMarket := catalogWithoutCrypto.MarketOf("")

	assert.Equal(t, vo.MarketCrypto, fallbackMarket.Value())
	assert.True(t, fallbackMarket.IsOpen(mustParseTime(t, "2026-09-13T21:00:00+08:00")))
	assert.True(t, catalogWithoutCrypto.IsRecognised("crypto"))
}

func TestASessionKeepsItsClockReadingOnADayThatLosesAnHour(t *testing.T) {
	// A market whose zone observes daylight saving still opens at nine on the morning
	// the clocks go forward — nine in the morning and nine hours after midnight are
	// different moments that day. Nothing in Taipei turns on this; the zone is a
	// setting, and the next market's might.
	//
	// London goes forward at 01:00 on 2026-03-29, so that day is twenty-three hours
	// long: adding nine hours to midnight lands at 10:00, not 09:00.
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

	// A window covering that whole local day, asked about in universal time.
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
