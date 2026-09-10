package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTelegramDeliveryDomainKeepsWhatWasMeantAndDropsWhatWasNot(t *testing.T) {
	testCases := []struct {
		name           string
		botToken       string
		chatID         string
		expectedToken  string
		expectedChatID string
	}{
		{
			name:           "an ordinary pair is kept as it was written",
			botToken:       "123456:AAHqwertyuiop1234",
			chatID:         "987654",
			expectedToken:  "123456:AAHqwertyuiop1234",
			expectedChatID: "987654",
		},
		{
			// A token pasted out of another window drags its surroundings along,
			// and nobody ever meant those to be part of it. This is the opposite
			// of a password, where the blanks are characters somebody chose.
			name:           "the blanks around a pasted value are not part of it",
			botToken:       "  123456:AAHqwertyuiop1234\n",
			chatID:         "\t987654 ",
			expectedToken:  "123456:AAHqwertyuiop1234",
			expectedChatID: "987654",
		},
		{
			name:           "a chat named rather than numbered is accepted",
			botToken:       "123456:AAHqwertyuiop1234",
			chatID:         "@my_alerts_channel",
			expectedToken:  "123456:AAHqwertyuiop1234",
			expectedChatID: "@my_alerts_channel",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deliverySetting, err := domains.NewTelegramDeliveryDomain(
				dto.TelegramDeliveryWriteDto{
					BotToken: testCase.botToken,
					ChatID:   testCase.chatID,
				})

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedToken, deliverySetting.BotToken())
			assert.Equal(t, testCase.expectedChatID, deliverySetting.ToEntity(1, "sealed").ChatID)
		})
	}
}

func TestNewTelegramDeliveryDomainRefusesAHalfGivenSetting(t *testing.T) {
	testCases := []struct {
		name            string
		botToken        string
		chatID          string
		expectedMessage string
	}{
		{
			name:            "no token at all",
			botToken:        "",
			chatID:          "987654",
			expectedMessage: "必須給一組機器人金鑰",
		},
		{
			name:            "a token of nothing but blanks",
			botToken:        "   ",
			chatID:          "987654",
			expectedMessage: "必須給一組機器人金鑰",
		},
		{
			name:            "no chat at all",
			botToken:        "123456:AAHqwertyuiop1234",
			chatID:          "",
			expectedMessage: "必須給一個聊天室代號",
		},
		{
			name:            "a chat of nothing but blanks",
			botToken:        "123456:AAHqwertyuiop1234",
			chatID:          "  ",
			expectedMessage: "必須給一個聊天室代號",
		},
		{
			// Both blank reports the token, because that is the harder half to get
			// right and they will have to go and find it either way.
			name:            "neither half given reports the token",
			botToken:        "",
			chatID:          "",
			expectedMessage: "必須給一組機器人金鑰",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewTelegramDeliveryDomain(dto.TelegramDeliveryWriteDto{
				BotToken: testCase.botToken,
				ChatID:   testCase.chatID,
			})

			require.ErrorIs(t, err, domains.ErrTelegramDeliveryValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

// The tail is the only part of a token anybody sees again. Four characters answers
// the one question people ask of a stored token — is that the one I pasted? — and
// reconstructs nothing.
func TestTelegramDeliveryDomainShowsOnlyTheTailOfTheToken(t *testing.T) {
	testCases := []struct {
		name         string
		botToken     string
		expectedTail string
	}{
		{
			name:         "an ordinary token shows its last four characters",
			botToken:     "123456:AAHqwertyuiop1234",
			expectedTail: "1234",
		},
		{
			name:         "a token of exactly five characters still shows four",
			botToken:     "abcde",
			expectedTail: "bcde",
		},
		{
			// The edge where "show the last four" quietly becomes "show all of
			// it". A real token is never this short; a rule that leaks on its own
			// edge is not a rule.
			name:         "a token no longer than the tail shows nothing at all",
			botToken:     "abcd",
			expectedTail: "",
		},
		{
			name:         "a token shorter than the tail shows nothing at all",
			botToken:     "ab",
			expectedTail: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deliverySetting, err := domains.NewTelegramDeliveryDomain(
				dto.TelegramDeliveryWriteDto{BotToken: testCase.botToken, ChatID: "987654"})
			require.NoError(t, err)

			assert.Equal(t, testCase.expectedTail,
				deliverySetting.ToEntity(1, "sealed").BotTokenTail)
		})
	}
}

// The row this model builds has nowhere to put a usable token: what goes in is the
// sealed form, handed in by whoever knows how to make one.
func TestTelegramDeliveryDomainToEntityStoresOnlyTheSealedToken(t *testing.T) {
	deliverySetting, err := domains.NewTelegramDeliveryDomain(dto.TelegramDeliveryWriteDto{
		BotToken: "123456:AAHqwertyuiop1234",
		ChatID:   "987654",
	})
	require.NoError(t, err)

	delivery := deliverySetting.ToEntity(7, "the-sealed-form")

	assert.Equal(t, uint(7), delivery.UserID)
	assert.Equal(t, "the-sealed-form", delivery.SealedBotToken)
	assert.NotContains(t, delivery.SealedBotToken, "AAHqwertyuiop")
	assert.False(t, strings.Contains(delivery.BotTokenTail, "AAHqwertyuiop"))
}
