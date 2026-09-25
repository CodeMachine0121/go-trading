package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const historyLookbackCeiling = 90

func TestKCandleHistoryLookbackIsAStretchOfWholeDays(t *testing.T) {
	testCases := []struct {
		name             string
		days             int
		expectedDuration time.Duration
	}{
		{name: "a single day", days: 1, expectedDuration: 24 * time.Hour},
		{name: "the month somebody usually asks for", days: 30, expectedDuration: 30 * 24 * time.Hour},
		{name: "right on the ceiling", days: historyLookbackCeiling,
			expectedDuration: historyLookbackCeiling * 24 * time.Hour},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lookback, lookbackError := domains.NewKCandleHistoryLookbackDomain(
				testCase.days, historyLookbackCeiling)

			require.NoError(t, lookbackError)
			assert.Equal(t, testCase.expectedDuration, lookback.Duration())
		})
	}
}

func TestKCandleHistoryLookbackRefusesAStretchThatIsNotOne(t *testing.T) {
	// Zero or negative days describe no stretch at all.
	testCases := []struct {
		name string
		days int
	}{
		{name: "no days at all", days: 0},
		{name: "backwards", days: -1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, lookbackError := domains.NewKCandleHistoryLookbackDomain(
				testCase.days, historyLookbackCeiling)

			require.ErrorIs(t, lookbackError, domains.ErrKCandleHistoryLookback)
		})
	}
}

func TestKCandleHistoryLookbackRefusalNamesTheCeiling(t *testing.T) {
	// The refusal must say what may be asked for instead.
	_, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		historyLookbackCeiling+1, historyLookbackCeiling)

	require.ErrorIs(t, lookbackError, domains.ErrKCandleHistoryLookback)
	assert.Contains(t, lookbackError.Error(), "90")
}
