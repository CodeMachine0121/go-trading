package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// deliveryOwnerID is whoever every one of these acts as. It comes from the proof of
// identity in real use, never from a request body.
const deliveryOwnerID = uint(7)

// configuredAt is written out literally so assertions state the requirement rather than repeat
// arithmetic.
var configuredAt = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

type telegramDeliveryApplicationUnderTest struct {
	telegramDeliveryApplication *application.TelegramDeliveryApplication
	telegramDeliveryRepository  *mocks.MockITelegramDeliveryRepository
	secretSealProxy             *mocks.MockISecretSealProxy
	messageDeliveryProxy        *mocks.MockIMessageDeliveryProxy
}

// newTelegramDeliveryApplicationUnderTest wires the real domain service and models, mocking only
// the store, the lock and the message sender.
func newTelegramDeliveryApplicationUnderTest(
	t *testing.T,
) telegramDeliveryApplicationUnderTest {
	mockController := gomock.NewController(t)
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(mockController)
	secretSealProxy := mocks.NewMockISecretSealProxy(mockController)
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(mockController)

	return telegramDeliveryApplicationUnderTest{
		telegramDeliveryApplication: application.NewTelegramDeliveryApplication(
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy)),
		telegramDeliveryRepository: telegramDeliveryRepository,
		secretSealProxy:            secretSealProxy,
		messageDeliveryProxy:       messageDeliveryProxy,
	}
}

// aStoredDelivery is a setting as it comes back out of the store.
func aStoredDelivery() entities.TelegramDelivery {
	return entities.TelegramDelivery{
		ID:             1,
		UserID:         deliveryOwnerID,
		SealedBotToken: "the-sealed-token",
		BotTokenTail:   "1234",
		ChatID:         "987654",
		CreatedAt:      configuredAt,
		UpdatedAt:      configuredAt,
	}
}

func TestTelegramDeliveryApplicationGetDeliverySetting(t *testing.T) {
	t.Run("a stored setting comes back without the token", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(aStoredDelivery(), nil)

		deliveryDto, err := fixture.telegramDeliveryApplication.GetDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.NoError(t, err)
		assert.True(t, deliveryDto.Configured)
		assert.Equal(t, "987654", deliveryDto.ChatID)
		assert.Equal(t, "1234", deliveryDto.BotTokenTail)
		assert.Equal(t, configuredAt, deliveryDto.ConfiguredAt)
	})

	// Reading a setting must never open the sealed token; the lock is not set up, so touching it
	// fails the test.
	t.Run("reading a setting never opens the token", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(aStoredDelivery(), nil)

		_, err := fixture.telegramDeliveryApplication.GetDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.NoError(t, err)
	})

	// Never having set one up is an ordinary state, not a failure.
	t.Run("having never set one up is not a failure", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)

		deliveryDto, err := fixture.telegramDeliveryApplication.GetDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.NoError(t, err)
		assert.False(t, deliveryDto.Configured)
		assert.Empty(t, deliveryDto.ChatID)
		assert.Empty(t, deliveryDto.BotTokenTail)
	})

	t.Run("storage being broken is reported as itself", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		storageFailure := errors.New("the database is not there")
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(entities.TelegramDelivery{}, storageFailure)

		_, err := fixture.telegramDeliveryApplication.GetDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrTelegramDeliveryNotConfigured)
	})
}

func TestTelegramDeliveryApplicationSaveDeliverySetting(t *testing.T) {
	t.Run("the token is sealed before anything is written", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.secretSealProxy.EXPECT().
			Seal("123456:AAHqwertyuiop1234").
			Return("the-sealed-token", nil)
		fixture.telegramDeliveryRepository.EXPECT().
			Upsert(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, delivery entities.TelegramDelivery) (
				entities.TelegramDelivery, error,
			) {
				assert.Equal(t, deliveryOwnerID, delivery.UserID)
				assert.Equal(t, "the-sealed-token", delivery.SealedBotToken)
				assert.Equal(t, "1234", delivery.BotTokenTail)
				assert.Equal(t, "987654", delivery.ChatID)
				assert.NotContains(t, delivery.SealedBotToken, "AAHqwertyuiop")

				delivery.UpdatedAt = configuredAt

				return delivery, nil
			})

		deliveryDto, err := fixture.telegramDeliveryApplication.SaveDeliverySetting(
			context.Background(), deliveryOwnerID,
			dto.TelegramDeliveryWriteDto{
				BotToken: "123456:AAHqwertyuiop1234",
				ChatID:   "987654",
			})

		require.NoError(t, err)
		assert.True(t, deliveryDto.Configured)
		assert.Equal(t, "1234", deliveryDto.BotTokenTail)
	})

	// Storing the token unsealed would work silently, so the refusal is the feature.
	t.Run("with nothing to lock the token, nothing is written at all", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.secretSealProxy.EXPECT().
			Seal(gomock.Any()).
			Return("", domains.ErrSecretSealUnavailable)

		_, err := fixture.telegramDeliveryApplication.SaveDeliverySetting(
			context.Background(), deliveryOwnerID,
			dto.TelegramDeliveryWriteDto{BotToken: "123456:AAH", ChatID: "987654"})

		// No Upsert is set up, so reaching the store fails the test.
		require.ErrorIs(t, err, domains.ErrSecretSealUnavailable)
	})

	t.Run("a half-given setting is refused before anything is locked", func(t *testing.T) {
		testCases := []struct {
			name            string
			botToken        string
			chatID          string
			expectedMessage string
		}{
			{name: "no token", botToken: "  ", chatID: "987654", expectedMessage: "必須給一組機器人金鑰"},
			{name: "no chat", botToken: "123456:AAH", chatID: "", expectedMessage: "必須給一個聊天室代號"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newTelegramDeliveryApplicationUnderTest(t)

				_, err := fixture.telegramDeliveryApplication.SaveDeliverySetting(
					context.Background(), deliveryOwnerID,
					dto.TelegramDeliveryWriteDto{
						BotToken: testCase.botToken,
						ChatID:   testCase.chatID,
					})

				// Neither the lock nor the store is set up, so touching either
				// fails the test.
				require.ErrorIs(t, err, domains.ErrTelegramDeliveryValidation)
				assert.Contains(t, err.Error(), testCase.expectedMessage)
			})
		}
	})

	t.Run("a write that fails is reported", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		writeFailure := errors.New("the database is not there")
		fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).Return("the-sealed-token", nil)
		fixture.telegramDeliveryRepository.EXPECT().
			Upsert(gomock.Any(), gomock.Any()).
			Return(entities.TelegramDelivery{}, writeFailure)

		_, err := fixture.telegramDeliveryApplication.SaveDeliverySetting(
			context.Background(), deliveryOwnerID,
			dto.TelegramDeliveryWriteDto{BotToken: "123456:AAH", ChatID: "987654"})

		require.ErrorIs(t, err, writeFailure)
	})
}

func TestTelegramDeliveryApplicationRemoveDeliverySetting(t *testing.T) {
	t.Run("removing takes away this person's setting", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			DeleteByUser(gomock.Any(), deliveryOwnerID).
			Return(nil)

		err := fixture.telegramDeliveryApplication.RemoveDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.NoError(t, err)
	})

	t.Run("a removal that fails is reported", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		deleteFailure := errors.New("the database is not there")
		fixture.telegramDeliveryRepository.EXPECT().
			DeleteByUser(gomock.Any(), deliveryOwnerID).
			Return(deleteFailure)

		err := fixture.telegramDeliveryApplication.RemoveDeliverySetting(
			context.Background(), deliveryOwnerID)

		require.ErrorIs(t, err, deleteFailure)
	})
}

func TestTelegramDeliveryApplicationSendTestMessage(t *testing.T) {
	t.Run("the message goes out as the stored bot into the stored chat", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(aStoredDelivery(), nil)
		fixture.secretSealProxy.EXPECT().
			Unseal("the-sealed-token").
			Return("123456:AAHqwertyuiop1234", nil)
		fixture.messageDeliveryProxy.EXPECT().
			Deliver(gomock.Any(),
				vo.MessageDeliveryCredentialVo{
					BotToken: "123456:AAHqwertyuiop1234",
					ChatID:   "987654",
				},
				"哈囉，這是一則測試").
			Return(vo.DeliveryFailureNone, nil)

		result, err := fixture.telegramDeliveryApplication.SendTestMessage(
			context.Background(), deliveryOwnerID,
			dto.TestMessageDto{Message: "  哈囉，這是一則測試  "})

		require.NoError(t, err)
		assert.True(t, result.Delivered)
		assert.Empty(t, result.FailureReason)
	})

	// Every refusal is a result rather than an error: finding out is the test button working.
	t.Run("each refusal comes back as its own reason", func(t *testing.T) {
		testCases := []struct {
			name           string
			reason         vo.DeliveryFailureReasonVo
			expectedReason string
		}{
			{name: "the token was rejected", reason: vo.DeliveryFailureCredentialRejected, expectedReason: "credentialRejected"},
			{name: "the chat is unknown", reason: vo.DeliveryFailureDestinationNotFound, expectedReason: "destinationNotFound"},
			{name: "nothing answered", reason: vo.DeliveryFailureUnreachable, expectedReason: "unreachable"},
			{name: "the answer never came in time", reason: vo.DeliveryFailureTimedOut, expectedReason: "timedOut"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newTelegramDeliveryApplicationUnderTest(t)
				fixture.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), deliveryOwnerID).
					Return(aStoredDelivery(), nil)
				fixture.secretSealProxy.EXPECT().
					Unseal(gomock.Any()).
					Return("123456:AAH", nil)
				fixture.messageDeliveryProxy.EXPECT().
					Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testCase.reason, nil)

				result, err := fixture.telegramDeliveryApplication.SendTestMessage(
					context.Background(), deliveryOwnerID, dto.TestMessageDto{Message: "哈囉"})

				require.NoError(t, err, "對方拒絕不是這一次請求的失敗")
				assert.False(t, result.Delivered)
				assert.Equal(t, testCase.expectedReason, result.FailureReason)
			})
		}
	})

	t.Run("without a setting there is nothing to send with", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)

		_, err := fixture.telegramDeliveryApplication.SendTestMessage(
			context.Background(), deliveryOwnerID, dto.TestMessageDto{Message: "哈囉"})

		require.ErrorIs(t, err, domains.ErrTelegramDeliveryNotConfigured)
	})

	t.Run("a message that cannot be sent is refused before anything is read", func(t *testing.T) {
		testCases := []struct {
			name            string
			message         string
			expectedMessage string
		}{
			{name: "nothing but blanks", message: "   ", expectedMessage: "訊息不得為空白"},
			{
				name:            "one character over the maximum",
				message:         strings.Repeat("字", 4097),
				expectedMessage: "一則訊息上限為 4096 個字元",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newTelegramDeliveryApplicationUnderTest(t)

				_, err := fixture.telegramDeliveryApplication.SendTestMessage(
					context.Background(), deliveryOwnerID,
					dto.TestMessageDto{Message: testCase.message})

				// Nothing is set up, so reading the setting, opening the token or
				// sending anything fails the test.
				require.ErrorIs(t, err, domains.ErrTelegramDeliveryValidation)
				assert.Contains(t, err.Error(), testCase.expectedMessage)
			})
		}
	})

	// An unopenable token is the system's problem; reporting it as rejected would send somebody to
	// replace a fine token.
	t.Run("a token that cannot be opened is not a rejected token", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(aStoredDelivery(), nil)
		fixture.secretSealProxy.EXPECT().
			Unseal("the-sealed-token").
			Return("", domains.ErrSecretSealUnavailable)

		result, err := fixture.telegramDeliveryApplication.SendTestMessage(
			context.Background(), deliveryOwnerID, dto.TestMessageDto{Message: "哈囉"})

		require.ErrorIs(t, err, domains.ErrSecretSealUnavailable)
		assert.Empty(t, result.FailureReason)
	})

	t.Run("the sending side failing is reported as an error", func(t *testing.T) {
		fixture := newTelegramDeliveryApplicationUnderTest(t)
		deliverFailure := errors.New("build message request failed")
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), deliveryOwnerID).
			Return(aStoredDelivery(), nil)
		fixture.secretSealProxy.EXPECT().Unseal(gomock.Any()).Return("123456:AAH", nil)
		fixture.messageDeliveryProxy.EXPECT().
			Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(vo.DeliveryFailureUnreachable, deliverFailure)

		_, err := fixture.telegramDeliveryApplication.SendTestMessage(
			context.Background(), deliveryOwnerID, dto.TestMessageDto{Message: "哈囉"})

		require.ErrorIs(t, err, deliverFailure)
	})
}
