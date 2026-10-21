package domains_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatisticTimesInUseAreTheOnesEachBarTakes(t *testing.T) {
	testCases := []struct {
		name             string
		declaredInterval string
		candleCount      int
		barOpenTimes     []string
		statisticTimes   []string
		expected         []string
	}{
		{
			name: "the latest before each close, one taken at a close going to the next bar", declaredInterval: "1h", candleCount: 3,
			barOpenTimes:   []string{"2026-09-03T05:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T07:00:00Z"},
			statisticTimes: []string{"2026-09-03T07:00:00Z", "2026-09-03T05:30:00Z", "2026-09-03T05:55:00Z", "2026-09-03T06:00:00Z"},
			expected:       []string{"2026-09-03T05:55:00Z", "2026-09-03T06:00:00Z", "2026-09-03T07:00:00Z"},
		},
		{
			name: "a bar whose latest is from an earlier bar takes none", declaredInterval: "1h", candleCount: 3,
			barOpenTimes:   []string{"2026-09-03T05:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T07:00:00Z"},
			statisticTimes: []string{"2026-09-03T05:30:00Z", "2026-09-03T05:55:00Z"},
			expected:       []string{"2026-09-03T05:55:00Z"},
		},
		{
			name: "one-minute bars share one taken within five minutes of their close", declaredInterval: "1m", candleCount: 3,
			barOpenTimes:   []string{"2026-09-03T08:34:00Z", "2026-09-03T08:35:00Z", "2026-09-03T08:36:00Z"},
			statisticTimes: []string{"2026-09-03T08:25:00Z", "2026-09-03T08:32:00Z"},
			expected:       []string{"2026-09-03T08:32:00Z"},
		},
		{
			name: "later one-minute bars more than five minutes past it take none", declaredInterval: "1m", candleCount: 3,
			barOpenTimes:   []string{"2026-09-03T08:34:00Z", "2026-09-03T08:35:00Z", "2026-09-03T08:36:00Z"},
			statisticTimes: []string{"2026-09-03T08:30:00Z"},
			expected:       []string{"2026-09-03T08:30:00Z"},
		},
		{
			name: "no statistics take none", declaredInterval: "1h", candleCount: 1,
			barOpenTimes: []string{"2026-09-03T07:00:00Z"}, statisticTimes: []string{}, expected: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			alignment, selectionError := calculationFor(t, testCase.declaredInterval, testCase.candleCount).
				SelectContractInput(contractCandlesAt(testCase.barOpenTimes...))
			require.NoError(t, selectionError)
			statisticTimes := make([]time.Time, 0, len(testCase.statisticTimes))
			for _, statisticTime := range testCase.statisticTimes {
				statisticTimes = append(statisticTimes, momentAt(statisticTime))
			}
			expected := make([]time.Time, 0, len(testCase.expected))
			for _, statisticTime := range testCase.expected {
				expected = append(expected, momentAt(statisticTime))
			}

			assert.Equal(t, expected, alignment.StatisticTimesInUse(statisticTimes))
		})
	}
}
