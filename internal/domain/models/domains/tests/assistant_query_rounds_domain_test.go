package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

// recordedRounds is a count that has already spent this many queries, so that a test
// about the boundary is not also a test about how the count is advanced.
func recordedRounds(limit int, spent int) domains.AssistantQueryRoundsDomain {
	rounds := domains.NewAssistantQueryRoundsDomain(limit)
	for range spent {
		rounds = rounds.Record()
	}

	return rounds
}

func TestAssistantQueryRoundsAllowsUpToTheLimitAndNoFurther(t *testing.T) {
	testCases := []struct {
		name                 string
		limit                int
		spent                int
		expectedAllows       bool
		expectedReachedLimit bool
	}{
		{name: "nothing spent yet", limit: 8, spent: 0, expectedAllows: true, expectedReachedLimit: false},
		{name: "part way through", limit: 8, spent: 3, expectedAllows: true, expectedReachedLimit: false},
		{
			name: "the last one is still allowed", limit: 8, spent: 7,
			expectedAllows: true, expectedReachedLimit: false,
		},
		{
			name: "spending the limit ends it", limit: 8, spent: 8,
			expectedAllows: false, expectedReachedLimit: true,
		},
		{
			name: "a limit of one allows exactly one", limit: 1, spent: 1,
			expectedAllows: false, expectedReachedLimit: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rounds := recordedRounds(testCase.limit, testCase.spent)

			assert.Equal(t, testCase.expectedAllows, rounds.Allows())
			assert.Equal(t, testCase.expectedReachedLimit, rounds.ReachedLimit())
			assert.Equal(t, testCase.spent, rounds.Used())
		})
	}
}

func TestAssistantQueryRoundsLeavesTheCountItWasAskedFromAlone(t *testing.T) {
	// Whether a query may run is asked several times within one answer, so recording
	// must not change the value the asker is holding.
	rounds := domains.NewAssistantQueryRoundsDomain(1)

	recorded := rounds.Record()

	assert.Equal(t, 0, rounds.Used())
	assert.True(t, rounds.Allows())
	assert.Equal(t, 1, recorded.Used())
	assert.False(t, recorded.Allows())
}
