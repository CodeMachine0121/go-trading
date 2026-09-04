package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssistantCandleLimitDomainHandsOverAtMostTheLimit(t *testing.T) {
	testCases := []struct {
		name              string
		limit             int
		requestedCount    int
		expectedCount     int
		expectedTruncated bool
	}{
		{
			name: "well within the limit", limit: 200, requestedCount: 50,
			expectedCount: 50, expectedTruncated: false,
		},
		{
			name: "one below the limit", limit: 200, requestedCount: 199,
			expectedCount: 199, expectedTruncated: false,
		},
		{
			name: "exactly the limit", limit: 200, requestedCount: 200,
			expectedCount: 200, expectedTruncated: false,
		},
		{
			name: "one above the limit is cut back and reported", limit: 200, requestedCount: 201,
			expectedCount: 200, expectedTruncated: true,
		},
		{
			name: "far above the limit is cut back and reported", limit: 200, requestedCount: 500,
			expectedCount: 200, expectedTruncated: true,
		},
		{
			// Not saying how many is not asking for everything, so nothing was
			// withheld and there is nothing to report.
			name: "naming no count means the limit itself", limit: 200, requestedCount: 0,
			expectedCount: 200, expectedTruncated: false,
		},
		{
			name: "a limit of one hands over one", limit: 1, requestedCount: 5,
			expectedCount: 1, expectedTruncated: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candleLimit, limitError := domains.NewAssistantCandleLimitDomain(
				testCase.limit, testCase.requestedCount)

			require.NoError(t, limitError)
			assert.Equal(t, testCase.expectedCount, candleLimit.Count())
			assert.Equal(t, testCase.expectedTruncated, candleLimit.Truncated())
		})
	}
}

func TestNewAssistantCandleLimitDomainRefusesACountBelowZero(t *testing.T) {
	_, limitError := domains.NewAssistantCandleLimitDomain(200, -1)

	require.ErrorIs(t, limitError, domains.ErrAssistantQueryArgument)
	assert.Contains(t, limitError.Error(), "必須大於零")
}
