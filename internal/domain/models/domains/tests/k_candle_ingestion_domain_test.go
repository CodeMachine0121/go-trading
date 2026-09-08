package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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

			// The bound is the spec's own number, not one asked of the code being
			// tested: derived from the coarsest interval it would widen by itself the
			// day a coarser one is added, and go on passing while meaning less.
			assert.Equal(t, testCase.expectedExtraSpan, unalignedStart.Sub(window.StartTime))
			assert.Less(t, unalignedStart.Sub(window.StartTime), 24*time.Hour,
				"多抓的量不會達到一整天——回補上限的原始目的（避免第一輪暴衝）靠這個上界成立")
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

func TestABackfillStartedAtAnEdgeProducesAWholeOldestBucket(t *testing.T) {
	// This is what the whole change is for, and it is the one claim neither half's
	// tests make on their own: the window's start is tested here, merging is tested
	// over in the series, and nothing joined them up. Joined up, a run that begins
	// where the window says produces an oldest bucket whose opening really is that
	// day's opening — not an afternoon's wearing the day's name.
	window := ingestionDomain(t, time.Date(2026, 8, 30, 14, 3, 0, 0, time.UTC), 5).
		BackfillWindow("BTCUSDT", vo.MarketCrypto, time.Time{})
	require.Equal(t, time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC), window.StartTime)

	// What the source hands back for that window, newest first: the day opens at the
	// window's own start, so the first candle of the bucket is the first of the day.
	dayOpeningPrice := decimal.NewFromInt(100)
	storedKCandles := []entities.KCandle{
		{
			Symbol: "BTCUSDT", OpenTime: window.StartTime.Add(time.Hour),
			Open: decimal.NewFromInt(300), High: decimal.NewFromInt(300),
			Low: decimal.NewFromInt(300), Close: decimal.NewFromInt(300),
			Volume: decimal.NewFromInt(1),
		},
		{
			Symbol: "BTCUSDT", OpenTime: window.StartTime,
			Open: dayOpeningPrice, High: decimal.NewFromInt(100),
			Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100),
			Volume: decimal.NewFromInt(1),
		},
	}

	dailyInterval, intervalError := domains.NewAggregationIntervalDomain("1d")
	require.NoError(t, intervalError)
	buckets := domains.NewKCandleSeriesDomain("BTCUSDT", dailyInterval, storedKCandles).Buckets()

	require.Len(t, buckets, 1)
	assert.Equal(t, window.StartTime, buckets[0].OpenTime(),
		"最舊那一格從當天第一分鐘算起")
	assert.True(t, buckets[0].ToDto().Open.Equal(dayOpeningPrice),
		"它的開盤價是那一天第一分鐘的開盤價，不是後來某一刻的")
}

func TestABucketIsWholeEvenWhenTheMarketOnlyTradedPartOfTheDay(t *testing.T) {
	// A symbol listed that morning, or a market that only opens for a few hours. The
	// bucket still belongs to the whole day and still holds everything that traded in
	// it — the alignment is about where fetching starts, not about demanding that a
	// day be busy. Refusing this would turn every listing day and every short session
	// into missing data.
	//
	// The day is taken from the window rather than written down, so this stays a
	// statement about what a backfill produces — which is what this file is about.
	window := ingestionDomain(t, time.Date(2026, 8, 30, 14, 3, 0, 0, time.UTC), 5).
		BackfillWindow("BTCUSDT", vo.MarketCrypto, time.Time{})
	dayStart := window.StartTime
	firstTradeOfTheDay := dayStart.Add(3 * time.Hour)
	storedKCandles := []entities.KCandle{
		{
			Symbol: "BTCUSDT", OpenTime: firstTradeOfTheDay.Add(time.Minute),
			Open: decimal.NewFromInt(210), High: decimal.NewFromInt(210),
			Low: decimal.NewFromInt(210), Close: decimal.NewFromInt(210),
			Volume: decimal.NewFromInt(1),
		},
		{
			Symbol: "BTCUSDT", OpenTime: firstTradeOfTheDay,
			Open: decimal.NewFromInt(200), High: decimal.NewFromInt(200),
			Low: decimal.NewFromInt(200), Close: decimal.NewFromInt(200),
			Volume: decimal.NewFromInt(1),
		},
	}

	dailyInterval, intervalError := domains.NewAggregationIntervalDomain("1d")
	require.NoError(t, intervalError)
	buckets := domains.NewKCandleSeriesDomain("BTCUSDT", dailyInterval, storedKCandles).Buckets()

	require.Len(t, buckets, 1)
	assert.Equal(t, dayStart, buckets[0].OpenTime(),
		"那一格仍然屬於一整天，起始時間是當天零點")
	assert.True(t, buckets[0].ToDto().Volume.Equal(decimal.NewFromInt(2)),
		"當天成交的每一根都在裡面，一根都沒少")
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
