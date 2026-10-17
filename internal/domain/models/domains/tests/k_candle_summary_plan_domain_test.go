package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var summaryPlanNow = time.Date(2026, 10, 18, 10, 20, 0, 0, time.UTC)

func summaryPlanAt(t *testing.T, interval string, startTime time.Time, endTime time.Time) domains.KCandleSummaryPlanDomain {
	t.Helper()

	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		dto.KCandleSeriesQueryDto{Symbol: "BTCUSDT", StartTime: startTime, EndTime: endTime, Interval: interval},
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}).
			MarketOf(string(vo.MarketCrypto)),
		1000,
	)
	require.NoError(t, validationError)

	return domains.NewKCandleSummaryPlanDomain(seriesQueryDomain, summaryPlanNow)
}

func summaryMoment(text string) time.Time {
	moment, parseError := time.Parse(time.RFC3339Nano, text)
	if parseError != nil {
		panic(parseError)
	}

	return moment
}

func TestSummaryPlanTakesWholeFinishedBucketsFromSummaries(t *testing.T) {
	testCases := []struct {
		name                    string
		interval                string
		startTime               string
		endTime                 string
		expectedSummaryInterval vo.AggregationIntervalVo
		expectedSummarizedStart string
		expectedSummarizedEnd   string
	}{
		{"nine hundred days of daily candles", "1d", "2024-04-25T16:17:48.450Z", "2026-10-12T16:17:48.450Z",
			vo.AggregationIntervalOneDay, "2024-04-26T00:00:00Z", "2026-10-12T00:00:00Z"},
		{"four hours come from hourly summaries", "4h", "2026-10-10T00:00:00Z", "2026-10-11T23:59:00Z",
			vo.AggregationIntervalOneHour, "2026-10-10T00:00:00Z", "2026-10-11T20:00:00Z"},
		{"daily candles up to now stop before today", "1d", "2026-10-10T00:00:00Z", "2026-10-18T10:20:00Z",
			vo.AggregationIntervalOneDay, "2026-10-10T00:00:00Z", "2026-10-18T00:00:00Z"},
		{"hourly candles up to now stop before today", "1h", "2026-10-17T00:00:00Z", "2026-10-18T10:20:00Z",
			vo.AggregationIntervalOneHour, "2026-10-17T00:00:00Z", "2026-10-18T00:00:00Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planDomain := summaryPlanAt(t, testCase.interval, summaryMoment(testCase.startTime), summaryMoment(testCase.endTime))

			assert.True(t, planDomain.UsesSummaries())
			assert.Equal(t, testCase.expectedSummaryInterval, planDomain.SummaryInterval())
			assert.Equal(t, summaryMoment(testCase.expectedSummarizedStart), planDomain.SummarizedStart())
			assert.Equal(t, summaryMoment(testCase.expectedSummarizedEnd), planDomain.SummarizedEnd())
		})
	}
}

func TestSummaryPlanMergesThePartialEndsFromMinutes(t *testing.T) {
	testCases := []struct {
		name              string
		startTime         string
		endTime           string
		expectedStretches [][2]string
	}{
		{"both ends partial", "2024-04-25T16:17:48.450Z", "2026-10-12T16:17:48.450Z", [][2]string{
			{"2024-04-25T16:17:48.450Z", "2024-04-25T23:59:59.999999Z"},
			{"2026-10-12T00:00:00Z", "2026-10-12T16:17:48.450Z"},
		}},
		{"start on a day edge leaves only today", "2026-10-10T00:00:00Z", "2026-10-18T10:20:00Z", [][2]string{
			{"2026-10-18T00:00:00Z", "2026-10-18T10:20:00Z"},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planDomain := summaryPlanAt(t, "1d", summaryMoment(testCase.startTime), summaryMoment(testCase.endTime))

			stretches := planDomain.MinuteStretches()

			require.Len(t, stretches, len(testCase.expectedStretches))
			for index, expectedStretch := range testCase.expectedStretches {
				assert.Equal(t, summaryMoment(expectedStretch[0]), stretches[index].StartTime())
				assert.Equal(t, summaryMoment(expectedStretch[1]), stretches[index].EndTime())
			}
		})
	}
}

func TestSummaryPlanKeepsMergingMinutesWhenNoWholeFinishedBucketFits(t *testing.T) {
	testCases := []struct {
		name      string
		interval  string
		startTime string
		endTime   string
	}{
		{"fifteen minutes over five days", "15m", "2026-10-13T00:00:00Z", "2026-10-18T00:00:00Z"},
		{"daily candles within today", "1d", "2026-10-18T00:30:00Z", "2026-10-18T10:20:00Z"},
		{"hourly candles inside one hour", "1h", "2026-10-17T09:05:00Z", "2026-10-17T09:55:00Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planDomain := summaryPlanAt(t, testCase.interval, summaryMoment(testCase.startTime), summaryMoment(testCase.endTime))

			assert.False(t, planDomain.UsesSummaries())
		})
	}
}

func TestSummaryPlanListsOnlyTheDaysNotYetSummarized(t *testing.T) {
	planDomain := summaryPlanAt(t, "1d", summaryMoment("2026-10-10T00:00:00Z"), summaryMoment("2026-10-14T00:00:00Z"))

	daysToSummarize := planDomain.DaysToSummarize([]time.Time{summaryMoment("2026-10-10T00:00:00Z"), summaryMoment("2026-10-12T00:00:00Z")})

	assert.Equal(t, []time.Time{summaryMoment("2026-10-11T00:00:00Z"), summaryMoment("2026-10-13T00:00:00Z")}, daysToSummarize)
}

func TestSummaryPlanIncludesOnlySummariesOfItsIntervalInsideItsStretch(t *testing.T) {
	planDomain := summaryPlanAt(t, "4h", summaryMoment("2026-10-10T04:00:00Z"), summaryMoment("2026-10-11T23:59:00Z"))

	testCases := []struct {
		name     string
		interval vo.AggregationIntervalVo
		openTime string
		expected bool
	}{
		{"an hour inside", vo.AggregationIntervalOneHour, "2026-10-10T04:00:00Z", true},
		{"a day of the same stretch", vo.AggregationIntervalOneDay, "2026-10-11T00:00:00Z", false},
		{"an hour before the stretch", vo.AggregationIntervalOneHour, "2026-10-10T03:00:00Z", false},
		{"the hour at the stretch end", vo.AggregationIntervalOneHour, "2026-10-11T20:00:00Z", false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, planDomain.IncludesSummary(testCase.interval, summaryMoment(testCase.openTime)))
		})
	}
}

func TestMinuteCapacityCountsEveryMinuteOfTheStretch(t *testing.T) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: summaryMoment("2026-10-10T00:00:00Z"), EndTime: summaryMoment("2026-10-10T23:59:59.999999Z"),
	})
	require.NoError(t, validationError)

	assert.Equal(t, 1440, queryDomain.MinuteCapacity())
}

func TestSummaryPlanMergesAMissingDayFromEveryMinuteOfIt(t *testing.T) {
	planDomain := summaryPlanAt(t, "1d", summaryMoment("2026-10-10T00:00:00Z"), summaryMoment("2026-10-14T00:00:00Z"))

	dayStretch := planDomain.DayStretch(summaryMoment("2026-10-11T00:00:00Z"))

	assert.Equal(t, "BTCUSDT", dayStretch.Symbol())
	assert.Equal(t, summaryMoment("2026-10-11T00:00:00Z"), dayStretch.StartTime())
	assert.Equal(t, summaryMoment("2026-10-11T23:59:59.999999Z"), dayStretch.EndTime())
	assert.Equal(t, 1440, dayStretch.MinuteCapacity())
}

func TestSummaryPlanNeverReadsMoreThanADayOfMinutesPerStretch(t *testing.T) {
	testCases := []struct {
		name      string
		interval  string
		startTime string
		endTime   string
	}{
		{"nine hundred days ending inside a day", "1d", "2024-04-25T16:17:48.450Z", "2026-10-12T16:17:48.450Z"},
		{"four hours up to now", "4h", "2026-09-01T03:10:00Z", "2026-10-18T10:20:00Z"},
		{"hours reaching into the future", "1h", "2026-10-01T00:00:00Z", "2026-11-05T00:00:00Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			planDomain := summaryPlanAt(t, testCase.interval, summaryMoment(testCase.startTime), summaryMoment(testCase.endTime))

			for _, stretch := range planDomain.MinuteStretches() {
				stretchEnd := stretch.EndTime()
				if stretchEnd.After(summaryPlanNow) {
					stretchEnd = summaryPlanNow
				}
				assert.LessOrEqual(t, int(stretchEnd.Sub(stretch.StartTime())/time.Minute)+1, 1440)
			}
		})
	}
}

func TestSummaryPlanProbesFromAMissingDayToTheSummarizedEnd(t *testing.T) {
	planDomain := summaryPlanAt(t, "1d", summaryMoment("2026-10-10T00:00:00Z"), summaryMoment("2026-10-14T00:00:00Z"))

	probeStretch := planDomain.StretchFromDay(summaryMoment("2026-10-11T00:00:00Z"))

	assert.Equal(t, summaryMoment("2026-10-11T00:00:00Z"), probeStretch.StartTime())
	assert.Equal(t, summaryMoment("2026-10-13T23:59:59.999999Z"), probeStretch.EndTime())
}

func TestStoredSummaryLengthsAreHoursAndDays(t *testing.T) {
	assert.Equal(t, []vo.AggregationIntervalVo{vo.AggregationIntervalOneHour, vo.AggregationIntervalOneDay},
		domains.KCandleSummaryIntervalValues())
}
