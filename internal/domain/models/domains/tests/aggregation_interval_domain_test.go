package domains_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, value)
	require.NoError(t, parseError)

	return parsedTime
}

func TestNewAggregationIntervalDomainReadsWhatWasDeclared(t *testing.T) {
	testCases := []struct {
		name          string
		declared      string
		expectedValue vo.AggregationIntervalVo
	}{
		{name: "one minute", declared: "1m", expectedValue: vo.AggregationIntervalOneMinute},
		{name: "five minutes", declared: "5m", expectedValue: vo.AggregationIntervalFiveMinutes},
		{name: "fifteen minutes", declared: "15m", expectedValue: vo.AggregationIntervalFifteenMinutes},
		{name: "one hour", declared: "1h", expectedValue: vo.AggregationIntervalOneHour},
		{name: "four hours", declared: "4h", expectedValue: vo.AggregationIntervalFourHours},
		{name: "one day", declared: "1d", expectedValue: vo.AggregationIntervalOneDay},
		{name: "surrounding blanks and letter case", declared: "  1H ", expectedValue: vo.AggregationIntervalOneHour},
		{
			name: "declaring nothing means one minute", declared: "",
			expectedValue: vo.AggregationIntervalOneMinute,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervalDomain, validationError := domains.NewAggregationIntervalDomain(testCase.declared)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedValue, intervalDomain.Value())
		})
	}
}

func TestNewAggregationIntervalDomainRefusesAnythingElse(t *testing.T) {
	testCases := []struct {
		name     string
		declared string
	}{
		{name: "a length nobody offers", declared: "7m"},
		{name: "a length that would not divide a day", declared: "7h"},
		{name: "not a length at all", declared: "hourly"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewAggregationIntervalDomain(testCase.declared)

			// The reason is all an interval owes anyone. Which sentinel it counts as
			// is asserted where the wrapping happens — see the series query and the
			// strategy, which refuse the same bad spelling under their own sentinels.
			require.Error(t, validationError)
			assert.Contains(t, validationError.Error(), "彙總刻度只能是 1m、5m、15m、1h、4h、1d 其中之一")
		})
	}
}

func TestAggregationIntervalDomainBucketStartCutsFromMidnight(t *testing.T) {
	testCases := []struct {
		name                string
		declared            string
		moment              string
		expectedBucketStart string
	}{
		{
			name: "five minutes leaves an aligned moment where it is", declared: "5m",
			moment: "2026-09-02T10:05:00Z", expectedBucketStart: "2026-09-02T10:05:00Z",
		},
		{
			name: "an hour swallows the minutes", declared: "1h",
			moment: "2026-09-02T10:35:00Z", expectedBucketStart: "2026-09-02T10:00:00Z",
		},
		{
			name: "the last five minutes of an hour still belong to that hour", declared: "1h",
			moment: "2026-09-02T10:55:00Z", expectedBucketStart: "2026-09-02T10:00:00Z",
		},
		{
			name: "a quarter of an hour", declared: "15m",
			moment: "2026-09-02T10:44:00Z", expectedBucketStart: "2026-09-02T10:30:00Z",
		},
		{
			name: "four hours are cut from midnight, not from the hour asked about", declared: "4h",
			moment: "2026-09-02T02:03:00Z", expectedBucketStart: "2026-09-02T00:00:00Z",
		},
		{
			name: "four hours later in the day", declared: "4h",
			moment: "2026-09-02T23:57:00Z", expectedBucketStart: "2026-09-02T20:00:00Z",
		},
		{
			name: "a day ends at midnight in universal time", declared: "1d",
			moment: "2026-09-02T23:55:00Z", expectedBucketStart: "2026-09-02T00:00:00Z",
		},
		{
			name: "the next day starts its own bucket", declared: "1d",
			moment: "2026-09-03T00:00:00Z", expectedBucketStart: "2026-09-03T00:00:00Z",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervalDomain, validationError := domains.NewAggregationIntervalDomain(testCase.declared)
			require.NoError(t, validationError)

			bucketStart := intervalDomain.BucketStart(mustParseTime(t, testCase.moment))

			assert.Equal(t, mustParseTime(t, testCase.expectedBucketStart), bucketStart)
		})
	}
}

// everyDeclarableInterval reads the whole declarable set out of the refusal an
// unrecognised spelling comes back with — the only place that set is exposed outside
// its own package.
//
// Reading prose in a test is worth it here: this is the enumeration the "every length
// divides a day" invariant has to be checked against, and a hard-coded copy would go
// quietly out of step with the set the moment anyone added a row.
func everyDeclarableInterval(t *testing.T) []string {
	t.Helper()

	_, refusal := domains.NewAggregationIntervalDomain("彙總刻度不可能是這個")
	require.Error(t, refusal)

	_, listed, found := strings.Cut(refusal.Error(), "彙總刻度只能是 ")
	require.True(t, found, "拒絕的說法變了，這個 helper 要跟著改")
	spellings, _, found := strings.Cut(listed, " 其中之一")
	require.True(t, found, "拒絕的說法變了，這個 helper 要跟著改")

	return strings.Split(spellings, "、")
}

func TestTheCoarsestIntervalIsADayAndItsEdgesServeEveryOtherInterval(t *testing.T) {
	// Anything aligning to a bucket edge asks for this one rather than naming a length
	// of its own. That is only sound because its edges are a subset of every other
	// interval's — which holds because each declarable length divides a day.
	t.Run("it is a day", func(t *testing.T) {
		assert.Equal(t, vo.AggregationIntervalOneDay,
			domains.NewCoarsestAggregationIntervalDomain().Value())
	})

	t.Run("its edge is also an edge for each of the six on offer", func(t *testing.T) {
		// This is the whole reason one alignment covers all of them.
		//
		// The spellings are read out of the refusal, which is the only place the set
		// is exposed. Hard-coding them here would mean a seventh length was simply
		// absent from the list rather than caught by it — the opposite of a guard.
		coarsestEdge := domains.NewCoarsestAggregationIntervalDomain().BucketStart(
			time.Date(2026, 9, 7, 14, 3, 0, 0, time.UTC))
		require.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), coarsestEdge)

		declarableSpellings := everyDeclarableInterval(t)
		require.Len(t, declarableSpellings, 6, "六種——多一種就要回來讀這條不變量")

		for _, declared := range declarableSpellings {
			intervalDomain, buildError := domains.NewAggregationIntervalDomain(declared)
			require.NoError(t, buildError)

			assert.Equal(t, coarsestEdge, intervalDomain.BucketStart(coarsestEdge),
				"最粗那一種的邊界必須也是 %s 的邊界", declared)
		}
	})

	t.Run("the oldest bucket of a coarse interval starts where a backfill did", func(t *testing.T) {
		// The acceptance criterion that says aligning once serves a coarser reading
		// too. The edge has to come from a real backfill: taken from BucketStart it
		// would only restate that truncation is idempotent, which the loop above
		// already says, and it would pass with the alignment removed entirely.
		ingestionDomain, ingestionError := domains.NewKCandleIngestionDomain(
			time.Date(2026, 9, 8, 14, 3, 0, 0, time.UTC), 5, 24*time.Hour)
		require.NoError(t, ingestionError)
		edge := ingestionDomain.
			BackfillWindow("BTCUSDT", vo.MarketCrypto, time.Time{}).StartTime

		fourHourInterval, buildError := domains.NewAggregationIntervalDomain("4h")
		require.NoError(t, buildError)

		buckets := domains.NewKCandleSeriesDomain("BTCUSDT", fourHourInterval,
			[]entities.KCandle{
				{
					Symbol: "BTCUSDT", OpenTime: edge.Add(time.Minute),
					Open: decimal.NewFromInt(110), High: decimal.NewFromInt(110),
					Low: decimal.NewFromInt(110), Close: decimal.NewFromInt(110),
					Volume: decimal.NewFromInt(1),
				},
				{
					Symbol: "BTCUSDT", OpenTime: edge,
					Open: decimal.NewFromInt(100), High: decimal.NewFromInt(100),
					Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100),
					Volume: decimal.NewFromInt(1),
				},
			}).Buckets()

		require.Len(t, buckets, 1)
		assert.Equal(t, edge, buckets[0].OpenTime())
	})
}

func TestAggregationIntervalDomainBucketCountIncludesBothEnds(t *testing.T) {
	testCases := []struct {
		name                string
		declared            string
		startTime           string
		endTime             string
		expectedBucketCount int
	}{
		{
			name: "one moment is one bucket", declared: "1h",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T10:00:00Z", expectedBucketCount: 1,
		},
		{
			name: "a range inside one bucket is still one bucket", declared: "1h",
			startTime: "2026-09-02T10:32:00Z", endTime: "2026-09-02T10:47:00Z", expectedBucketCount: 1,
		},
		{
			name: "crossing an edge makes it two", declared: "1h",
			startTime: "2026-09-02T09:58:00Z", endTime: "2026-09-02T10:30:00Z", expectedBucketCount: 2,
		},
		{
			name: "five minutes over a day", declared: "5m",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-02T23:55:00Z", expectedBucketCount: 288,
		},
		{
			name: "the same day at one day is one bucket", declared: "1d",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-02T23:55:00Z", expectedBucketCount: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervalDomain, validationError := domains.NewAggregationIntervalDomain(testCase.declared)
			require.NoError(t, validationError)

			bucketCount := intervalDomain.BucketCount(
				mustParseTime(t, testCase.startTime), mustParseTime(t, testCase.endTime))

			assert.Equal(t, testCase.expectedBucketCount, bucketCount)
		})
	}
}

func TestAggregationIntervalDomainSourceCandleCountBoundsWhatABucketCanHold(t *testing.T) {
	testCases := []struct {
		name                      string
		declared                  string
		bucketCount               int
		expectedSourceCandleCount int
	}{
		{name: "one minute holds one candle per bucket", declared: "1m", bucketCount: 1000, expectedSourceCandleCount: 1000},
		{name: "five minutes holds five", declared: "5m", bucketCount: 100, expectedSourceCandleCount: 500},
		{name: "a quarter of an hour holds fifteen", declared: "15m", bucketCount: 10, expectedSourceCandleCount: 150},
		{name: "an hour holds sixty", declared: "1h", bucketCount: 24, expectedSourceCandleCount: 1440},
		{name: "a day holds one thousand four hundred and forty", declared: "1d", bucketCount: 2, expectedSourceCandleCount: 2880},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervalDomain, validationError := domains.NewAggregationIntervalDomain(testCase.declared)
			require.NoError(t, validationError)

			assert.Equal(t,
				testCase.expectedSourceCandleCount,
				intervalDomain.SourceCandleCount(testCase.bucketCount))
		})
	}
}
