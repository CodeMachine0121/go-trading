package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyLookbackCeiling is the ceiling these cases are written against. It is a
// setting rather than domain knowledge, so it comes in from outside.
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
	// Zero days is not a small request, it is a request that does not hold together:
	// there is no stretch to fetch. Negative is the same thing said backwards.
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
	// Somebody turned away has to know what to ask for instead. A refusal that only
	// says "too far" leaves them halving the number until something works.
	_, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		historyLookbackCeiling+1, historyLookbackCeiling)

	require.ErrorIs(t, lookbackError, domains.ErrKCandleHistoryLookback)
	assert.Contains(t, lookbackError.Error(), "90")
}
