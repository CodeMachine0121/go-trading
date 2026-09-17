package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedAggregationIntervalDomainAnswers(t *testing.T) {
	testCases := []struct {
		name                 string
		aggregationIntervals []string
		expectedShared       string
		expectedNamed        []string
	}{
		{
			name:                 "all the same is that one",
			aggregationIntervals: []string{"1h", "1h", "1h"},
			expectedShared:       "1h",
		},
		{
			name: "one on its own cannot disagree with anybody",
			// The rule is about a set. A set of one has nothing to compare against,
			// so whichever coarseness it reads is the shared one.
			aggregationIntervals: []string{"5m"},
			expectedShared:       "5m",
		},
		{
			name:                 "two that differ are refused, and both are named",
			aggregationIntervals: []string{"1h", "5m"},
			expectedNamed:        []string{"1h", "5m"},
		},
		{
			name: "every one it found is named, each once",
			// Naming "1h" twice would read as four sources with four opinions.
			aggregationIntervals: []string{"1h", "1d", "1h", "5m"},
			expectedNamed:        []string{"1h", "1d", "5m"},
		},
		{
			name:                 "blanks around a coarseness do not make it a different one",
			aggregationIntervals: []string{" 1h", "1h "},
			expectedShared:       "1h",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shared, sharedError := domains.NewSharedAggregationIntervalDomain(
				testCase.aggregationIntervals).Shared()

			if testCase.expectedShared != "" {
				require.NoError(t, sharedError)
				assert.Equal(t, testCase.expectedShared, shared)

				return
			}

			require.Error(t, sharedError)
			for _, expectedInterval := range testCase.expectedNamed {
				assert.ErrorContains(t, sharedError, expectedInterval)
			}
		})
	}
}

// Nothing at all is not a shared anything. It is refused rather than answered with
// an empty coarseness, which would travel on as a query nobody could serve.
func TestSharedAggregationIntervalDomainRefusesAnEmptySet(t *testing.T) {
	_, sharedError := domains.NewSharedAggregationIntervalDomain(nil).Shared()

	require.Error(t, sharedError)
	assert.ErrorContains(t, sharedError, "沒有任何信號來源")
}
