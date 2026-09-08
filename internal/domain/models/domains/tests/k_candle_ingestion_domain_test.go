package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const backfillLookback = 24 * time.Hour

func at(hour int, minute int, second int) time.Time {
	return time.Date(2026, 8, 30, hour, minute, second, 0, time.UTC)
}

func ingestionDomain(t *testing.T, currentTime time.Time, roundCandleCount int) domains.KCandleIngestionDomain {
	t.Helper()

	ingestionDomain, buildError := domains.NewKCandleIngestionDomain(
		currentTime, roundCandleCount, backfillLookback)
	require.NoError(t, buildError)

	return ingestionDomain
}

func TestLatestClosedOpenTimeExcludesTheCandleStillRunning(t *testing.T) {
	testCases := []struct {
		name                 string
		currentTime          time.Time
		expectedLatestClosed time.Time
	}{
		{
			name:                 "part way through an interval",
			currentTime:          at(9, 7, 20),
			expectedLatestClosed: at(9, 6, 0),
		},
		{
			name:                 "one second before the interval finishes",
			currentTime:          at(9, 7, 59),
			expectedLatestClosed: at(9, 6, 0),
		},
		{
			name:                 "exactly on the mark, so the interval just finished",
			currentTime:          at(9, 8, 0),
			expectedLatestClosed: at(9, 7, 0),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			latestClosed := ingestionDomain(t, testCase.currentTime, 5).LatestClosedOpenTime()

			assert.Equal(t, testCase.expectedLatestClosed, latestClosed)
		})
	}
}

func TestScheduledWindowCoversTheNewestClosedCandlesBackwards(t *testing.T) {
	testCases := []struct {
		name              string
		roundCandleCount  int
		expectedStartTime time.Time
		expectedEndTime   time.Time
	}{
		{
			name:              "five candles",
			roundCandleCount:  5,
			expectedStartTime: at(9, 2, 0),
			expectedEndTime:   at(9, 6, 0),
		},
		{
			name:              "a single candle",
			roundCandleCount:  1,
			expectedStartTime: at(9, 6, 0),
			expectedEndTime:   at(9, 6, 0),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, at(9, 7, 0), testCase.roundCandleCount).
				ScheduledWindow("BTCUSDT", vo.MarketCrypto)

			assert.Equal(t, "BTCUSDT", window.Symbol)
			assert.Equal(t, testCase.expectedStartTime, window.StartTime)
			assert.Equal(t, testCase.expectedEndTime, window.EndTime)
			assert.False(t, window.IsEmpty())
		})
	}
}

func TestBackfillWindowStartsAfterTheStoredCandleOrAtTheEdgeTheLookbackReaches(t *testing.T) {
	testCases := []struct {
		name                 string
		latestStoredOpenTime time.Time
		expectedStartTime    time.Time
		expectedEmpty        bool
	}{
		{
			name:                 "gap inside the lookback starts right after the stored candle",
			latestStoredOpenTime: at(7, 0, 0),
			expectedStartTime:    at(7, 1, 0),
		},
		{
			// Reaching back by the lookback lands at 08-29 09:07, which is mid-bucket at
			// every coarseness above a minute. It starts at that day's edge instead.
			name:                 "gap wider than the lookback starts at the edge of the day it reaches",
			latestStoredOpenTime: time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC),
			expectedStartTime:    time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:                 "never held a candle starts at the edge of the day the lookback reaches",
			latestStoredOpenTime: time.Time{},
			expectedStartTime:    time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:                 "a gap of exactly one candle",
			latestStoredOpenTime: at(9, 5, 0),
			expectedStartTime:    at(9, 6, 0),
		},
		{
			name:                 "no gap at all comes back empty",
			latestStoredOpenTime: at(9, 6, 0),
			expectedStartTime:    at(9, 7, 0),
			expectedEmpty:        true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, at(9, 7, 0), 5).
				BackfillWindow("BTCUSDT", vo.MarketCrypto, testCase.latestStoredOpenTime)

			assert.Equal(t, "BTCUSDT", window.Symbol)
			assert.Equal(t, testCase.expectedStartTime, window.StartTime)
			assert.Equal(t, at(9, 6, 0), window.EndTime)
			assert.Equal(t, testCase.expectedEmpty, window.IsEmpty())
		})
	}
}

func TestBackfillReachesBackToABucketEdgeWhateverTimeItIsAsked(t *testing.T) {
	// A stretch that begins mid-bucket makes the oldest bucket of every coarseness
	// begin part way through itself, and it is still merged and handed over as a whole
	// one — at one day, an afternoon's opening price and half a day's volume reported
	// as the day's. Nothing downstream can tell that from a market that traded little,
	// so the start has to be right.
	//
	// Every case reaches back 24 hours and then down to that day's edge, so they all
	// land on the same moment however far into the day they were asked.
	testCases := []struct {
		name        string
		currentTime time.Time
	}{
		{
			name:        "asked part way through the day",
			currentTime: time.Date(2026, 8, 30, 14, 3, 0, 0, time.UTC),
		},
		{
			name:        "asked exactly on the edge, which moves nothing",
			currentTime: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "asked in the last minute of the day, reaching back furthest",
			currentTime: time.Date(2026, 8, 30, 23, 59, 0, 0, time.UTC),
		},
		{
			name:        "asked in the first minute of the day, reaching back least",
			currentTime: time.Date(2026, 8, 30, 0, 1, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, testCase.currentTime, 5).
				BackfillWindow("BTCUSDT", vo.MarketCrypto, time.Time{})

			assert.Equal(t,
				time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC), window.StartTime)
		})
	}
}

func TestBackfillNeverReachesBackMoreThanOneBucketBeyondTheLookback(t *testing.T) {
	// The lookback exists to stop the first round after a long silence from bolting.
	// Reaching down to an edge loosens that, so how much it loosens it has to be
	// bounded — and the bound is one bucket of the coarsest coarseness, never more.
	testCases := []struct {
		name              string
		currentTime       time.Time
		expectedExtraSpan time.Duration
	}{
		{
			name:              "part way through the day",
			currentTime:       time.Date(2026, 8, 30, 14, 3, 0, 0, time.UTC),
			expectedExtraSpan: 14*time.Hour + 3*time.Minute,
		},
		{
			name:              "the last minute of the day is the worst case",
			currentTime:       time.Date(2026, 8, 30, 23, 59, 0, 0, time.UTC),
			expectedExtraSpan: 23*time.Hour + 59*time.Minute,
		},
		{
			name:              "on the edge nothing extra is fetched",
			currentTime:       time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
			expectedExtraSpan: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, testCase.currentTime, 5).
				BackfillWindow("BTCUSDT", vo.MarketCrypto, time.Time{})

			unalignedStart := testCase.currentTime.Add(-backfillLookback)

			assert.Equal(t, testCase.expectedExtraSpan, unalignedStart.Sub(window.StartTime))
			// The bound is one bucket of the coarsest coarseness, so it is derived
			// rather than written down: the day a coarser interval is added, this
			// keeps checking what the test's name says it checks.
			coarsestBucketSpan := time.Duration(
				domains.NewCoarsestAggregationIntervalDomain().SourceCandleCount(1)) *
				domains.KCandleInterval
			assert.Less(t, unalignedStart.Sub(window.StartTime), coarsestBucketSpan,
				"多抓的量不會達到一整格")
		})
	}
}

func TestBackfillLeavesAStartTakenFromStoredDataAlone(t *testing.T) {
	// That start already sits on a minute edge and abuts the candles that are there.
	// Reaching down to a bucket edge would step back over them and fetch what is
	// already stored — so the rounding must not touch this one.
	window := ingestionDomain(t, time.Date(2026, 8, 30, 14, 3, 0, 0, time.UTC), 5).
		BackfillWindow("BTCUSDT", vo.MarketCrypto,
			time.Date(2026, 8, 30, 12, 30, 0, 0, time.UTC))

	assert.Equal(t, time.Date(2026, 8, 30, 12, 31, 0, 0, time.UTC), window.StartTime,
		"接在已存那一根之後，沒有被拉回當天零點")
}

func TestSelectClosedDropsTheCandleStillRunning(t *testing.T) {
	reported := []vo.MarketKCandleVo{
		{Symbol: "BTCUSDT", OpenTime: at(9, 5, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 6, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 7, 0)},
	}

	closed := ingestionDomain(t, at(9, 7, 20), 5).SelectClosed(reported)

	assert.Equal(t, []time.Time{at(9, 5, 0), at(9, 6, 0)}, openTimesOf(closed))
}

func TestSelectClosedKeepsEveryCandleWhenAllHaveFinished(t *testing.T) {
	reported := []vo.MarketKCandleVo{
		{Symbol: "BTCUSDT", OpenTime: at(9, 5, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 6, 0)},
	}

	closed := ingestionDomain(t, at(9, 7, 20), 5).SelectClosed(reported)

	assert.Equal(t, []time.Time{at(9, 5, 0), at(9, 6, 0)}, openTimesOf(closed))
}

func TestNewKCandleIngestionDomainRejectsACandleCountOfZeroOrLess(t *testing.T) {
	_, buildError := domains.NewKCandleIngestionDomain(at(9, 7, 0), 0, backfillLookback)

	require.ErrorIs(t, buildError, domains.ErrKCandleIngestionValidation)
	assert.Contains(t, buildError.Error(), "單輪取回根數必須大於零")
}

func TestKCandleFetchWindowVoReportsWhetherItCoversAnything(t *testing.T) {
	testCases := []struct {
		name          string
		startTime     time.Time
		endTime       time.Time
		expectedEmpty bool
	}{
		{
			name:          "start after end covers nothing",
			startTime:     at(9, 5, 0),
			endTime:       at(9, 0, 0),
			expectedEmpty: true,
		},
		{
			name:          "start equal to end still covers one candle",
			startTime:     at(9, 0, 0),
			endTime:       at(9, 0, 0),
			expectedEmpty: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, testCase.startTime, testCase.endTime)

			assert.Equal(t, testCase.expectedEmpty, window.IsEmpty())
		})
	}
}

func TestMarketKCandleVoCarriesEveryFigureIntoTheWriteShape(t *testing.T) {
	marketKCandle := vo.MarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            at(9, 0, 0).In(time.FixedZone("UTC+8", 8*60*60)),
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
	}

	writeDto := marketKCandle.ToWriteDto()

	assert.Equal(t, "BTCUSDT", writeDto.Symbol)
	assert.Equal(t, at(9, 0, 0), writeDto.OpenTime)
	assert.Equal(t, "100", writeDto.Open.String())
	assert.Equal(t, "120", writeDto.High.String())
	assert.Equal(t, "90", writeDto.Low.String())
	assert.Equal(t, "110", writeDto.Close.String())
	assert.Equal(t, "11", writeDto.Volume.String())
	assert.Equal(t, "1200", writeDto.QuoteVolume.Decimal.String())
	assert.Equal(t, "5", writeDto.TakerBuyBaseVolume.Decimal.String())
	assert.Equal(t, "600", writeDto.TakerBuyQuoteVolume.Decimal.String())
}

func openTimesOf(marketKCandles []vo.MarketKCandleVo) []time.Time {
	openTimes := make([]time.Time, 0, len(marketKCandles))
	for _, marketKCandle := range marketKCandles {
		openTimes = append(openTimes, marketKCandle.OpenTime)
	}

	return openTimes
}
