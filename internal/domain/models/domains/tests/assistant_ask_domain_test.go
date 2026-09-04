package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssistantAskDomainKeepsTheQuestionWithoutTheBlanksAroundIt(t *testing.T) {
	testCases := []struct {
		name             string
		question         string
		expectedQuestion string
	}{
		{
			name:             "an ordinary question is kept as it was written",
			question:         "BTCUSDT 最近走勢如何",
			expectedQuestion: "BTCUSDT 最近走勢如何",
		},
		{
			name:             "the blanks around a question are not part of it",
			question:         "  BTCUSDT 最近走勢  ",
			expectedQuestion: "BTCUSDT 最近走勢",
		},
		{
			name:             "full-width blanks are blanks too",
			question:         "　全形空白包住　",
			expectedQuestion: "全形空白包住",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assistantAskDomain, validationError := domains.NewAssistantAskDomain(testCase.question)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedQuestion, assistantAskDomain.Question())
		})
	}
}

func TestNewAssistantAskDomainRefusesAQuestionThatSaidNothing(t *testing.T) {
	testCases := []struct {
		name     string
		question string
	}{
		{name: "nothing at all", question: ""},
		{name: "spaces only", question: "   "},
		{name: "tabs and newlines only", question: "\t\n "},
		{name: "full-width blanks only", question: "　"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewAssistantAskDomain(testCase.question)

			require.ErrorIs(t, validationError, domains.ErrAssistantAskEmpty)
			assert.Contains(t, validationError.Error(), "必須寫點什麼")
		})
	}
}
