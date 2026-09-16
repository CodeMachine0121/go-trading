package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestMessageDomainAcceptsAMessageThatCanBeSent(t *testing.T) {
	testCases := []struct {
		name            string
		message         string
		expectedMessage string
	}{
		{
			name:            "an ordinary message",
			message:         "這是一則來自 go-trading 的測試訊息",
			expectedMessage: "這是一則來自 go-trading 的測試訊息",
		},
		{
			name:            "the blanks around a message are not part of it",
			message:         "  哈囉  \n",
			expectedMessage: "哈囉",
		},
		{
			name:            "exactly the maximum length is not too long",
			message:         strings.Repeat("字", 4096),
			expectedMessage: strings.Repeat("字", 4096),
		},
		{
			// Characters, not bytes. Four thousand Chinese characters is three
			// times as many bytes, and it is characters that a message holds.
			name:            "a message counted in characters rather than bytes",
			message:         strings.Repeat("字", 2000),
			expectedMessage: strings.Repeat("字", 2000),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testMessage, err := domains.NewTestMessageDomain(testCase.message)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedMessage, testMessage.Value())
		})
	}
}

func TestNewTestMessageDomainRefusesAMessageThatCannotBeSent(t *testing.T) {
	testCases := []struct {
		name            string
		message         string
		expectedMessage string
	}{
		{name: "nothing at all", message: "", expectedMessage: "訊息不得為空白"},
		{name: "nothing but blanks", message: "   \n\t ", expectedMessage: "訊息不得為空白"},
		{
			// Refused rather than trimmed, for the same reason an over-long
			// password is refused: somebody told it worked would believe all of it
			// arrived.
			name:            "one character over the maximum",
			message:         strings.Repeat("字", 4097),
			expectedMessage: "一則訊息上限為 4096 個字元",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewTestMessageDomain(testCase.message)

			require.ErrorIs(t, err, domains.ErrTelegramDeliveryValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}
