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
			currentTime:          at(9, 7, 0),
			expectedLatestClosed: at(9, 0, 0),
		},
		{
			name:                 "one second before the interval finishes",
			currentTime:          at(9, 9, 59),
			expectedLatestClosed: at(9, 0, 0),
		},
		{
			name:                 "exactly on the mark, so the interval just finished",
			currentTime:          at(9, 10, 0),
			expectedLatestClosed: at(9, 5, 0),
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
			expectedStartTime: at(8, 40, 0),
			expectedEndTime:   at(9, 0, 0),
		},
		{
			name:              "a single candle",
			roundCandleCount:  1,
			expectedStartTime: at(9, 0, 0),
			expectedEndTime:   at(9, 0, 0),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, at(9, 7, 0), testCase.roundCandleCount).
				ScheduledWindow("BTCUSDT")

			assert.Equal(t, "BTCUSDT", window.Symbol)
			assert.Equal(t, testCase.expectedStartTime, window.StartTime)
			assert.Equal(t, testCase.expectedEndTime, window.EndTime)
			assert.False(t, window.IsEmpty())
		})
	}
}

func TestBackfillWindowStartsAfterTheStoredCandleButNeverBeyondTheLookback(t *testing.T) {
	testCases := []struct {
		name                 string
		latestStoredOpenTime time.Time
		expectedStartTime    time.Time
		expectedEmpty        bool
	}{
		{
			name:                 "gap inside the lookback starts right after the stored candle",
			latestStoredOpenTime: at(7, 0, 0),
			expectedStartTime:    at(7, 5, 0),
		},
		{
			name:                 "gap wider than the lookback starts at the lookback",
			latestStoredOpenTime: time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC),
			expectedStartTime:    time.Date(2026, 8, 29, 9, 7, 0, 0, time.UTC),
		},
		{
			name:                 "never held a candle fills the whole lookback",
			latestStoredOpenTime: time.Time{},
			expectedStartTime:    time.Date(2026, 8, 29, 9, 7, 0, 0, time.UTC),
		},
		{
			name:                 "a gap of exactly one candle",
			latestStoredOpenTime: at(8, 55, 0),
			expectedStartTime:    at(9, 0, 0),
		},
		{
			name:                 "no gap at all comes back empty",
			latestStoredOpenTime: at(9, 0, 0),
			expectedStartTime:    at(9, 5, 0),
			expectedEmpty:        true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			window := ingestionDomain(t, at(9, 7, 0), 5).
				BackfillWindow("BTCUSDT", testCase.latestStoredOpenTime)

			assert.Equal(t, "BTCUSDT", window.Symbol)
			assert.Equal(t, testCase.expectedStartTime, window.StartTime)
			assert.Equal(t, at(9, 0, 0), window.EndTime)
			assert.Equal(t, testCase.expectedEmpty, window.IsEmpty())
		})
	}
}

func TestSelectClosedDropsTheCandleStillRunning(t *testing.T) {
	reported := []vo.MarketKCandleVo{
		{Symbol: "BTCUSDT", OpenTime: at(8, 55, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 0, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 5, 0)},
	}

	closed := ingestionDomain(t, at(9, 7, 0), 5).SelectClosed(reported)

	assert.Equal(t, []time.Time{at(8, 55, 0), at(9, 0, 0)}, openTimesOf(closed))
}

func TestSelectClosedKeepsEveryCandleWhenAllHaveFinished(t *testing.T) {
	reported := []vo.MarketKCandleVo{
		{Symbol: "BTCUSDT", OpenTime: at(8, 55, 0)},
		{Symbol: "BTCUSDT", OpenTime: at(9, 0, 0)},
	}

	closed := ingestionDomain(t, at(9, 7, 0), 5).SelectClosed(reported)

	assert.Equal(t, []time.Time{at(8, 55, 0), at(9, 0, 0)}, openTimesOf(closed))
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
			window := vo.NewKCandleFetchWindowVo("BTCUSDT", testCase.startTime, testCase.endTime)

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
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}

	writeDto := marketKCandle.ToWriteDto()

	assert.Equal(t, "BTCUSDT", writeDto.Symbol)
	assert.Equal(t, at(9, 0, 0), writeDto.OpenTime)
	assert.Equal(t, "100", writeDto.Open.String())
	assert.Equal(t, "120", writeDto.High.String())
	assert.Equal(t, "90", writeDto.Low.String())
	assert.Equal(t, "110", writeDto.Close.String())
	assert.Equal(t, "11", writeDto.Volume.String())
	assert.Equal(t, "1200", writeDto.QuoteVolume.String())
	assert.Equal(t, "5", writeDto.TakerBuyBaseVolume.String())
	assert.Equal(t, "600", writeDto.TakerBuyQuoteVolume.String())
}

func openTimesOf(marketKCandles []vo.MarketKCandleVo) []time.Time {
	openTimes := make([]time.Time, 0, len(marketKCandles))
	for _, marketKCandle := range marketKCandles {
		openTimes = append(openTimes, marketKCandle.OpenTime)
	}

	return openTimes
}
