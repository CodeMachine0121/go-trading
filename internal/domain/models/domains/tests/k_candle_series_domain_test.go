package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildSeriesKCandle names only the open time, because grouping and ordering are the
// behavior under test; the figures are there so a merged candle has something to say.
func buildSeriesKCandle(t *testing.T, openTime string, closePrice string) entities.KCandle {
	t.Helper()

	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            mustParseTime(t, openTime),
		Open:                decimal.NewFromInt(100),
		High:                decimal.NewFromInt(110),
		Low:                 decimal.NewFromInt(90),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.NewFromInt(1),
		QuoteVolume:         decimal.NewFromInt(1),
		TakerBuyBaseVolume:  decimal.NewFromInt(1),
		TakerBuyQuoteVolume: decimal.NewFromInt(1),
	}
}

func buildSeriesDomain(t *testing.T, declaredInterval string, kCandles []entities.KCandle) domains.KCandleSeriesDomain {
	t.Helper()

	intervalDomain, validationError := domains.NewAggregationIntervalDomain(declaredInterval)
	require.NoError(t, validationError)

	return domains.NewKCandleSeriesDomain("BTCUSDT", intervalDomain, kCandles)
}

func TestKCandleSeriesDomainPutsEachCandleInItsOwnBucket(t *testing.T) {
	testCases := []struct {
		name                 string
		declaredInterval     string
		openTimes            []string
		expectedBucketStarts []string
	}{
		{
			name: "candles either side of an hour edge become two candles", declaredInterval: "1h",
			openTimes:            []string{"2026-09-02T10:55:00Z", "2026-09-02T11:00:00Z"},
			expectedBucketStarts: []string{"2026-09-02T10:00:00Z", "2026-09-02T11:00:00Z"},
		},
		{
			name: "five minutes leaves every candle as its own", declaredInterval: "5m",
			openTimes: []string{"2026-09-02T10:00:00Z", "2026-09-02T10:05:00Z", "2026-09-02T10:10:00Z"},
			expectedBucketStarts: []string{
				"2026-09-02T10:00:00Z", "2026-09-02T10:05:00Z", "2026-09-02T10:10:00Z",
			},
		},
		{
			name: "candles either side of midnight become two days", declaredInterval: "1d",
			openTimes:            []string{"2026-09-02T23:55:00Z", "2026-09-03T00:00:00Z"},
			expectedBucketStarts: []string{"2026-09-02T00:00:00Z", "2026-09-03T00:00:00Z"},
		},
		{
			name: "a bucket nothing fell into is absent from the series", declaredInterval: "1h",
			openTimes:            []string{"2026-09-02T10:05:00Z", "2026-09-02T12:30:00Z"},
			expectedBucketStarts: []string{"2026-09-02T10:00:00Z", "2026-09-02T12:00:00Z"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			kCandles := make([]entities.KCandle, 0, len(testCase.openTimes))
			for _, openTime := range testCase.openTimes {
				kCandles = append(kCandles, buildSeriesKCandle(t, openTime, "105"))
			}

			seriesDto := buildSeriesDomain(t, testCase.declaredInterval, kCandles).ToDto()

			actualBucketStarts := make([]string, 0, len(seriesDto.KCandles))
			for _, aggregatedKCandle := range seriesDto.KCandles {
				actualBucketStarts = append(actualBucketStarts, aggregatedKCandle.OpenTime.Format("2006-01-02T15:04:05Z"))
			}
			assert.Equal(t, testCase.expectedBucketStarts, actualBucketStarts)
		})
	}
}

func TestKCandleSeriesDomainRunsEarliestFirstWhateverOrderItRead(t *testing.T) {
	seriesDto := buildSeriesDomain(t, "1h", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T12:00:00Z", "105"),
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "105"),
		buildSeriesKCandle(t, "2026-09-02T11:00:00Z", "105"),
	}).ToDto()

	assert.Equal(t,
		[]string{"2026-09-02T10:00:00Z", "2026-09-02T11:00:00Z", "2026-09-02T12:00:00Z"},
		[]string{
			seriesDto.KCandles[0].OpenTime.Format("2006-01-02T15:04:05Z"),
			seriesDto.KCandles[1].OpenTime.Format("2006-01-02T15:04:05Z"),
			seriesDto.KCandles[2].OpenTime.Format("2006-01-02T15:04:05Z"),
		})
}

func TestKCandleSeriesDomainMergesEveryCandleThatSharesABucket(t *testing.T) {
	seriesDto := buildSeriesDomain(t, "1h", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "120"),
		buildSeriesKCandle(t, "2026-09-02T10:05:00Z", "110"),
	}).ToDto()

	require.Len(t, seriesDto.KCandles, 1)
	assert.True(t, decimal.NewFromInt(110).Equal(seriesDto.KCandles[0].Close),
		"the bucket closes where its latest candle closed")
	assert.True(t, decimal.NewFromInt(2).Equal(seriesDto.KCandles[0].Volume),
		"the bucket traded what both candles traded")
}

func TestKCandleSeriesDomainReadingNothingIsAnEmptySeries(t *testing.T) {
	seriesDto := buildSeriesDomain(t, "1h", []entities.KCandle{}).ToDto()

	assert.Empty(t, seriesDto.KCandles)
	assert.Equal(t, "BTCUSDT", seriesDto.Symbol)
	assert.Equal(t, "1h", seriesDto.Interval)
}

func TestKCandleSeriesDomainNamesTheIntervalItWasCutAt(t *testing.T) {
	seriesDto := buildSeriesDomain(t, "", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "105"),
	}).ToDto()

	assert.Equal(t, "5m", seriesDto.Interval, "declaring nothing means five minutes")
}

func TestKCandleSeriesDomainBucketsAreHandedOutEarliestFirst(t *testing.T) {
	// Whoever computes from a series reads it in time order, and whoever takes the
	// latest few takes them off the end. Both depend on this order being the
	// series' own guarantee rather than the order the candles were read in.
	seriesDomain := buildSeriesDomain(t, "1h", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T12:00:00Z", "120"),
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "100"),
		buildSeriesKCandle(t, "2026-09-02T11:00:00Z", "110"),
	})

	buckets := seriesDomain.Buckets()

	require.Len(t, buckets, 3)
	assert.Equal(t, mustParseTime(t, "2026-09-02T10:00:00Z"), buckets[0].ToDto().OpenTime)
	assert.Equal(t, mustParseTime(t, "2026-09-02T11:00:00Z"), buckets[1].ToDto().OpenTime)
	assert.Equal(t, mustParseTime(t, "2026-09-02T12:00:00Z"), buckets[2].ToDto().OpenTime)
}

func TestKCandleSeriesDomainBucketsLeaveOutTheStretchesNothingFellInto(t *testing.T) {
	// Nothing is invented for the hour in between: an invented candle reads exactly
	// like a real one, and a market that did not trade is not a market that traded
	// flat. Whoever counts buckets therefore counts the ones that exist.
	seriesDomain := buildSeriesDomain(t, "1h", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "100"),
		buildSeriesKCandle(t, "2026-09-02T12:00:00Z", "120"),
	})

	buckets := seriesDomain.Buckets()

	require.Len(t, buckets, 2)
	assert.Equal(t, mustParseTime(t, "2026-09-02T10:00:00Z"), buckets[0].ToDto().OpenTime)
	assert.Equal(t, mustParseTime(t, "2026-09-02T12:00:00Z"), buckets[1].ToDto().OpenTime)
}

func TestKCandleSeriesDomainBucketsGatherEveryCandleOfTheSameStretch(t *testing.T) {
	seriesDomain := buildSeriesDomain(t, "1h", []entities.KCandle{
		buildSeriesKCandle(t, "2026-09-02T10:00:00Z", "100"),
		buildSeriesKCandle(t, "2026-09-02T10:05:00Z", "105"),
		buildSeriesKCandle(t, "2026-09-02T10:55:00Z", "155"),
	})

	buckets := seriesDomain.Buckets()

	require.Len(t, buckets, 1)
	assert.True(t, decimal.RequireFromString("155").Equal(buckets[0].ToDto().Close),
		"三根都併進同一格，收盤價來自最晚那一根")
}

func TestKCandleSeriesDomainBucketsOfNoCandlesIsEmptyRatherThanNothing(t *testing.T) {
	seriesDomain := buildSeriesDomain(t, "1h", []entities.KCandle{})

	buckets := seriesDomain.Buckets()

	assert.NotNil(t, buckets)
	assert.Empty(t, buckets)
}
