package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intervalOf(t *testing.T, declared vo.AggregationIntervalVo) domains.AggregationIntervalDomain {
	t.Helper()
	interval, intervalError := domains.NewAggregationIntervalDomain(string(declared))
	require.NoError(t, intervalError)

	return interval
}

func TestReplayPaceIsTheFinestSourceInterval(t *testing.T) {
	testCases := []struct {
		name              string
		sourceIntervals   []vo.AggregationIntervalVo
		expectedPace      vo.AggregationIntervalVo
		expectedCoarsest  vo.AggregationIntervalVo
		expectedMixesThem bool
	}{
		{
			name:              "a day, four hours and fifteen minutes pace at fifteen minutes",
			sourceIntervals:   []vo.AggregationIntervalVo{vo.AggregationIntervalOneDay, vo.AggregationIntervalFourHours, vo.AggregationIntervalFifteenMinutes},
			expectedPace:      vo.AggregationIntervalFifteenMinutes,
			expectedCoarsest:  vo.AggregationIntervalOneDay,
			expectedMixesThem: true,
		},
		{
			name:              "a day and four hours pace at four hours",
			sourceIntervals:   []vo.AggregationIntervalVo{vo.AggregationIntervalOneDay, vo.AggregationIntervalFourHours},
			expectedPace:      vo.AggregationIntervalFourHours,
			expectedCoarsest:  vo.AggregationIntervalOneDay,
			expectedMixesThem: true,
		},
		{
			name:              "all on one hour pace at one hour",
			sourceIntervals:   []vo.AggregationIntervalVo{vo.AggregationIntervalOneHour, vo.AggregationIntervalOneHour},
			expectedPace:      vo.AggregationIntervalOneHour,
			expectedCoarsest:  vo.AggregationIntervalOneHour,
			expectedMixesThem: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervals := []domains.AggregationIntervalDomain{}
			lookbackCounts := []int{}
			for _, declared := range testCase.sourceIntervals {
				intervals = append(intervals, intervalOf(t, declared))
				lookbackCounts = append(lookbackCounts, 0)
			}

			pace := domains.NewReplayPaceDomain(intervals, lookbackCounts)

			assert.Equal(t, testCase.expectedPace, pace.Interval().Value())
			assert.Equal(t, testCase.expectedCoarsest, pace.CoarsestInterval().Value())
			assert.Equal(t, testCase.expectedMixesThem, pace.MixesIntervals())
		})
	}
}

func TestReplayPaceCountsEachLookbackInItsOwnInterval(t *testing.T) {
	testCases := []struct {
		name            string
		sourceIntervals []vo.AggregationIntervalVo
		lookbackCounts  []int
		expectedWarmup  int
	}{
		{
			name:            "eighty days outweigh a hundred quarter hours",
			sourceIntervals: []vo.AggregationIntervalVo{vo.AggregationIntervalOneDay, vo.AggregationIntervalFifteenMinutes},
			lookbackCounts:  []int{80, 100},
			expectedWarmup:  80 * 96,
		},
		{
			name:            "on one interval the warm-up is the largest look-back",
			sourceIntervals: []vo.AggregationIntervalVo{vo.AggregationIntervalFifteenMinutes, vo.AggregationIntervalFifteenMinutes},
			lookbackCounts:  []int{100, 40},
			expectedWarmup:  100,
		},
		{
			name:            "nothing declared needs no warm-up",
			sourceIntervals: []vo.AggregationIntervalVo{vo.AggregationIntervalOneDay, vo.AggregationIntervalOneHour},
			lookbackCounts:  []int{0, 0},
			expectedWarmup:  0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			intervals := []domains.AggregationIntervalDomain{}
			for _, declared := range testCase.sourceIntervals {
				intervals = append(intervals, intervalOf(t, declared))
			}

			pace := domains.NewReplayPaceDomain(intervals, testCase.lookbackCounts)

			assert.Equal(t, testCase.expectedWarmup, pace.WarmupBarCount())
		})
	}
}

func TestReplayPaceHearsACoarserSourceOnlyFromItsFinishedBars(t *testing.T) {
	day := intervalOf(t, vo.AggregationIntervalOneDay)
	quarterHour := intervalOf(t, vo.AggregationIntervalFifteenMinutes)
	pace := domains.NewReplayPaceDomain(
		[]domains.AggregationIntervalDomain{day, quarterHour}, []int{0, 0})

	marchSecond := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	marchThird := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	dailyOpenTimes := []time.Time{marchSecond, marchThird}
	dailySignals := []domains.SignalDomain{
		domains.NewSignalDomainOf(vo.SignalBuy), domains.NewSignalDomainOf(vo.SignalSell),
	}

	testCases := []struct {
		name           string
		paceBarOpen    time.Time
		expectedSignal vo.SignalVo
	}{
		{
			name:           "the first quarter of a new day hears the day just finished",
			paceBarOpen:    marchThird,
			expectedSignal: vo.SignalBuy,
		},
		{
			name:           "the last quarter of a day closes that very day",
			paceBarOpen:    marchThird.Add(23*time.Hour + 45*time.Minute),
			expectedSignal: vo.SignalSell,
		},
		{
			name:           "the quarter before the last still hears the day before",
			paceBarOpen:    marchThird.Add(23*time.Hour + 30*time.Minute),
			expectedSignal: vo.SignalBuy,
		},
		{
			name:           "before any daily bar has finished it holds",
			paceBarOpen:    marchSecond.Add(23*time.Hour + 30*time.Minute),
			expectedSignal: vo.SignalHold,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			signals := pace.SignalsAtPace(day, dailyOpenTimes, dailySignals, []time.Time{testCase.paceBarOpen})

			require.Len(t, signals, 1)
			assert.Equal(t, testCase.expectedSignal, signals[0].Value())
		})
	}
}
