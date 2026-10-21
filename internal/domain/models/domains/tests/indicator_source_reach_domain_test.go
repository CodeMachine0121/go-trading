package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

func reachStretch(reachDomain domains.IndicatorSourceReachDomain) [2]time.Time {
	rangeQuery := reachDomain.SeriesQuery().RangeQuery()

	return [2]time.Time{rangeQuery.StartTime(), rangeQuery.EndTime()}
}

func summaryRangeOf(startTime string, endTime string) [2]time.Time {
	return [2]time.Time{momentAt(startTime), summaryMoment(endTime)}
}

func contractCandlesAt(openTimes ...string) []entities.KCandleContract {
	kCandleContracts := make([]entities.KCandleContract, 0, len(openTimes))
	for _, openTime := range openTimes {
		kCandleContracts = append(kCandleContracts, entities.KCandleContract{Symbol: "BTCUSDT", OpenTime: momentAt(openTime)})
	}

	return kCandleContracts
}

func TestSourceReachReadsSummariesOnlyForIntervalsThatHaveThem(t *testing.T) {
	testCases := []struct {
		declaredInterval string
		expected         bool
	}{
		{"1m", false}, {"5m", false}, {"15m", false}, {"1h", true}, {"4h", true}, {"1d", true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.declaredInterval, func(t *testing.T) {
			assert.Equal(t, testCase.expected, calculationFor(t, testCase.declaredInterval, 3).SourceReach().ReadsSummaries())
		})
	}
}

func TestSourceReachKeepsTheCountedReadForIntervalsWithoutSummaries(t *testing.T) {
	calculationDomain := calculationFor(t, "15m", 3)

	reachDomain := calculationDomain.SourceReach()

	assert.Equal(t, "BTCUSDT", reachDomain.Symbol())
	assert.Equal(t, momentAt("2026-09-03T08:30:00Z"), reachDomain.Cutoff())
	assert.Equal(t, 60, reachDomain.SourceCandleLimit())
}

func TestSourceReachStartsWithTheBucketsAskedForRightBeforeTheCutoff(t *testing.T) {
	testCases := []struct {
		name             string
		declaredInterval string
		candleCount      int
		expectedStretch  [2]time.Time
	}{
		{"three hours before the running hour", "1h", 3,
			summaryRangeOf("2026-09-03T05:00:00Z", "2026-09-03T07:59:59.999999Z")},
		{"two days before today", "1d", 2,
			summaryRangeOf("2026-09-01T00:00:00Z", "2026-09-02T23:59:59.999999Z")},
		{"two four hour buckets before the running one", "4h", 2,
			summaryRangeOf("2026-09-03T00:00:00Z", "2026-09-03T07:59:59.999999Z")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reachDomain := calculationFor(t, testCase.declaredInterval, testCase.candleCount).SourceReach()

			assert.Equal(t, testCase.expectedStretch, reachStretch(reachDomain))
		})
	}
}

func TestSourceReachIsCoveredOnceEnoughBucketsHoldCandles(t *testing.T) {
	testCases := []struct {
		name      string
		openTimes []string
		expected  bool
	}{
		{"three minutes in two hours are two buckets", []string{
			"2026-09-03T07:59:00Z", "2026-09-03T07:00:00Z", "2026-09-03T05:30:00Z"}, false},
		{"one minute in each of three hours", []string{
			"2026-09-03T07:59:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:30:00Z"}, true},
		{"nothing read", []string{}, false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reachDomain := calculationFor(t, "1h", 3).SourceReach().
				CountingKCandles(storedCandlesNewestFirst(testCase.openTimes...))

			assert.Equal(t, testCase.expected, reachDomain.IsCovered())
		})
	}
}

func TestSourceReachCountsContractCandlesTheSameWay(t *testing.T) {
	reachDomain := calculationFor(t, "1h", 3).SourceReach().
		CountingKCandleContracts(contractCandlesAt("2026-09-03T07:59:00Z", "2026-09-03T07:00:00Z", "2026-09-03T05:30:00Z"))

	assert.False(t, reachDomain.IsCovered())

	reachDomain = reachDomain.ReachingBackTo(momentAt("2026-09-03T02:17:00Z")).
		CountingKCandleContracts(contractCandlesAt("2026-09-03T02:17:00Z"))

	assert.True(t, reachDomain.IsCovered())
}

func TestSourceReachLooksForEarlierCandlesBeforeEverythingRead(t *testing.T) {
	reachDomain := calculationFor(t, "1h", 3).SourceReach()

	assert.Equal(t, momentAt("2026-09-03T05:00:00Z"), reachDomain.EarlierCutoff())
}

func TestSourceReachJumpsToTheEarlierCandlesBucketSizedByHowDenseTheLastStretchWas(t *testing.T) {
	testCases := []struct {
		name               string
		declaredInterval   string
		candleCount        int
		firstStretchOpens  []string
		earlierOpenTime    string
		expectedStretch    [2]time.Time
		expectedNextCutoff string
	}{
		{
			name: "two of three hours found leaves one, stretched by three over two", declaredInterval: "1h", candleCount: 3,
			firstStretchOpens:  []string{"2026-09-03T07:00:00Z", "2026-09-03T05:00:00Z"},
			earlierOpenTime:    "2026-09-03T02:17:00Z",
			expectedStretch:    summaryRangeOf("2026-09-03T01:00:00Z", "2026-09-03T02:59:59.999999Z"),
			expectedNextCutoff: "2026-09-03T01:00:00Z",
		},
		{
			name: "an empty stretch reaches back exactly the buckets still missing", declaredInterval: "1h", candleCount: 3,
			firstStretchOpens:  []string{},
			earlierOpenTime:    "2026-08-01T10:30:00Z",
			expectedStretch:    summaryRangeOf("2026-08-01T08:00:00Z", "2026-08-01T10:59:59.999999Z"),
			expectedNextCutoff: "2026-08-01T08:00:00Z",
		},
		{
			name: "a market closed on weekends reaches further than the days still missing", declaredInterval: "1d", candleCount: 10,
			firstStretchOpens: []string{
				"2026-09-02T01:00:00Z", "2026-09-01T01:00:00Z", "2026-08-31T01:00:00Z", "2026-08-28T01:00:00Z",
				"2026-08-27T01:00:00Z", "2026-08-26T01:00:00Z", "2026-08-25T01:00:00Z"},
			earlierOpenTime:    "2026-08-21T05:29:00Z",
			expectedStretch:    summaryRangeOf("2026-08-17T00:00:00Z", "2026-08-21T23:59:59.999999Z"),
			expectedNextCutoff: "2026-08-17T00:00:00Z",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reachDomain := calculationFor(t, testCase.declaredInterval, testCase.candleCount).SourceReach().
				CountingKCandles(storedCandlesNewestFirst(testCase.firstStretchOpens...)).
				ReachingBackTo(momentAt(testCase.earlierOpenTime))

			assert.Equal(t, testCase.expectedStretch, reachStretch(reachDomain))
			assert.Equal(t, momentAt(testCase.expectedNextCutoff), reachDomain.EarlierCutoff())
			assert.False(t, reachDomain.IsCovered())
		})
	}
}
