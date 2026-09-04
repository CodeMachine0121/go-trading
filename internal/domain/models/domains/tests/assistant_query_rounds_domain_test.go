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
	if spent > 0 {
		rounds = rounds.Record(spent)
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
			assert.Equal(t, testCase.limit-testCase.spent, rounds.Remaining())
		})
	}
}

func TestAssistantQueryRoundsLeavesTheCountItWasAskedFromAlone(t *testing.T) {
	// Whether a query may run is asked several times within one answer, so recording
	// must not change the value the asker is holding.
	rounds := domains.NewAssistantQueryRoundsDomain(1)

	recorded := rounds.Record(1)

	assert.Equal(t, 0, rounds.Used())
	assert.True(t, rounds.Allows())
	assert.Equal(t, 1, recorded.Used())
	assert.False(t, recorded.Allows())
}

func TestAssistantQueryRoundsRemainingNeverGoesBelowNone(t *testing.T) {
	// 助手一口氣要五次而只剩兩次時，誠實的答案是「前兩次」——
	// 負數會讓那個「前幾次」的切法變成一段恐慌。
	rounds := domains.NewAssistantQueryRoundsDomain(2).Record(5)

	assert.Equal(t, 0, rounds.Remaining())
	assert.False(t, rounds.Allows())
}

func TestAssistantQueryRoundsRecordsAWholeRoundAtOnce(t *testing.T) {
	// 一輪裡的三次查詢是三次，不是一次——不然一個回答可以在八輪裡查上幾十次。
	rounds := domains.NewAssistantQueryRoundsDomain(8).Record(3)

	assert.Equal(t, 3, rounds.Used())
	assert.Equal(t, 5, rounds.Remaining())
}
