package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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
		{name: "five minutes", declared: "5m", expectedValue: vo.AggregationIntervalFiveMinutes},
		{name: "fifteen minutes", declared: "15m", expectedValue: vo.AggregationIntervalFifteenMinutes},
		{name: "one hour", declared: "1h", expectedValue: vo.AggregationIntervalOneHour},
		{name: "four hours", declared: "4h", expectedValue: vo.AggregationIntervalFourHours},
		{name: "one day", declared: "1d", expectedValue: vo.AggregationIntervalOneDay},
		{name: "surrounding blanks and letter case", declared: "  1H ", expectedValue: vo.AggregationIntervalOneHour},
		{
			name: "declaring nothing means five minutes", declared: "",
			expectedValue: vo.AggregationIntervalFiveMinutes,
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

			require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
			assert.Contains(t, validationError.Error(), "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一")
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
		{name: "five minutes holds one candle per bucket", declared: "5m", bucketCount: 1000, expectedSourceCandleCount: 1000},
		{name: "a quarter of an hour holds three", declared: "15m", bucketCount: 10, expectedSourceCandleCount: 30},
		{name: "an hour holds twelve", declared: "1h", bucketCount: 24, expectedSourceCandleCount: 288},
		{name: "a day holds two hundred and eighty-eight", declared: "1d", bucketCount: 2, expectedSourceCandleCount: 576},
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
