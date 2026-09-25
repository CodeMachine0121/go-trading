package controller_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type telegramDeliveryRouterUnderTest struct {
	engine                     *gin.Engine
	telegramDeliveryRepository *mocks.MockITelegramDeliveryRepository
	secretSealProxy            *mocks.MockISecretSealProxy
	messageDeliveryProxy       *mocks.MockIMessageDeliveryProxy
}

func newTelegramDeliveryRouterUnderTest(t *testing.T) telegramDeliveryRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(mockController)
	secretSealProxy := mocks.NewMockISecretSealProxy(mockController)
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(mockController)

	telegramDeliveryController := controller.NewTelegramDeliveryController(
		application.NewTelegramDeliveryApplication(
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy)))

	// The real auth middleware is mounted as in production, since whose setting it is comes from the proof, not the body.
	requiresSignIn := doorOpenFor(t, signedInViewerID)

	engine := gin.New()
	engine.GET("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.GetDeliverySetting)
	engine.PUT("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.SaveDeliverySetting)
	engine.DELETE("/users/me/telegram-delivery",
		requiresSignIn, telegramDeliveryController.RemoveDeliverySetting)
	engine.POST("/users/me/telegram-delivery/test-message",
		requiresSignIn, telegramDeliveryController.SendTestMessage)

	return telegramDeliveryRouterUnderTest{
		engine:                     engine,
		telegramDeliveryRepository: telegramDeliveryRepository,
		secretSealProxy:            secretSealProxy,
		messageDeliveryProxy:       messageDeliveryProxy,
	}
}

// send makes a signed-in request unless signedIn is false.
func (fixture telegramDeliveryRouterUnderTest) send(
	method string, path string, body string, signedIn bool,
) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if signedIn {
		request.Header.Set("Authorization", signedInProof)
	}
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func aStoredDeliveryFor(userID uint) entities.TelegramDelivery {
	return entities.TelegramDelivery{
		ID:             1,
		UserID:         userID,
		SealedBotToken: "the-sealed-token",
		BotTokenTail:   "1234",
		ChatID:         "987654",
		UpdatedAt:      time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC),
	}
}

func TestTelegramDeliveryRouterGetDeliverySetting(t *testing.T) {
	t.Run("a stored setting answers with the tail and never the token", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(aStoredDeliveryFor(signedInViewerID), nil)

		response := fixture.send(http.MethodGet, "/users/me/telegram-delivery", "", true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"botTokenTail":"1234"`)
		assert.Contains(t, response.Body.String(), `"chatId":"987654"`)
		assert.NotContains(t, response.Body.String(), "the-sealed-token")
	})

	// Not 404: having no setting is the resource's ordinary state, not a missing resource.
	t.Run("having never set one up answers with a setting that says so", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)

		response := fixture.send(http.MethodGet, "/users/me/telegram-delivery", "", true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"configured":false`)
	})

	// Storage being unreachable must not read as "not set up", or someone would re-paste a token that is already stored.
	t.Run("storage failing is reported as the system's problem", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.TelegramDelivery{}, errors.New("the database is not there"))

		response := fixture.send(http.MethodGet, "/users/me/telegram-delivery", "", true)

		require.Equal(t, http.StatusBadGateway, response.Code)
		assert.NotContains(t, response.Body.String(), `"configured":false`)
	})

	t.Run("without a proof of identity the handler never runs", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodGet, "/users/me/telegram-delivery", "", false)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})
}

func TestTelegramDeliveryRouterSaveDeliverySetting(t *testing.T) {
	t.Run("a stored setting answers with what may be shown again", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.secretSealProxy.EXPECT().
			Seal("123456:AAHqwertyuiop1234").
			Return("the-sealed-token", nil)
		fixture.telegramDeliveryRepository.EXPECT().
			Upsert(gomock.Any(), gomock.Any()).
			Return(aStoredDeliveryFor(signedInViewerID), nil)

		response := fixture.send(http.MethodPut, "/users/me/telegram-delivery",
			`{"botToken":"123456:AAHqwertyuiop1234","chatId":"987654"}`, true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"botTokenTail":"1234"`)
		assert.NotContains(t, response.Body.String(), "AAHqwertyuiop")
	})

	t.Run("a half-given setting is a bad request", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/telegram-delivery",
			`{"botToken":"  ","chatId":"987654"}`, true)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "必須給一組機器人金鑰")
	})

	// The token was fine; saying otherwise would have them generating new ones.
	t.Run("nothing to lock the token with is the system being unavailable", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.secretSealProxy.EXPECT().
			Seal(gomock.Any()).
			Return("", domains.ErrSecretSealUnavailable)

		response := fixture.send(http.MethodPut, "/users/me/telegram-delivery",
			`{"botToken":"123456:AAH","chatId":"987654"}`, true)

		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Contains(t, response.Body.String(), "無法安全保存")
	})

	t.Run("a body that is not readable is a bad request", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/telegram-delivery", `{`, true)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("storage failing is reported as the system's problem", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).Return("the-sealed-token", nil)
		fixture.telegramDeliveryRepository.EXPECT().
			Upsert(gomock.Any(), gomock.Any()).
			Return(entities.TelegramDelivery{}, errors.New("the database is not there"))

		response := fixture.send(http.MethodPut, "/users/me/telegram-delivery",
			`{"botToken":"123456:AAH","chatId":"987654"}`, true)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func TestTelegramDeliveryRouterRemoveDeliverySetting(t *testing.T) {
	// 204 either way: the system can no longer reach them regardless.
	t.Run("removing answers with no content, with or without a setting", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			DeleteByUser(gomock.Any(), signedInViewerID).
			Return(nil)

		response := fixture.send(http.MethodDelete, "/users/me/telegram-delivery", "", true)

		require.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.String())
	})

	t.Run("storage failing is reported as the system's problem", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			DeleteByUser(gomock.Any(), signedInViewerID).
			Return(errors.New("the database is not there"))

		response := fixture.send(http.MethodDelete, "/users/me/telegram-delivery", "", true)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func TestTelegramDeliveryRouterSendTestMessage(t *testing.T) {
	// expectStoredSettingOpened stubs everything sending touches, for tests about the answer.
	expectStoredSettingOpened := func(fixture telegramDeliveryRouterUnderTest) {
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(aStoredDeliveryFor(signedInViewerID), nil)
		fixture.secretSealProxy.EXPECT().
			Unseal("the-sealed-token").
			Return("123456:AAHqwertyuiop1234", nil)
	}

	t.Run("a message that went through says so", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		expectStoredSettingOpened(fixture)
		fixture.messageDeliveryProxy.EXPECT().
			Deliver(gomock.Any(), gomock.Any(), "哈囉").
			Return(vo.DeliveryFailureNone, nil)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"delivered":true`)
		assert.NotContains(t, response.Body.String(), "failureReason")
	})

	// A refusal still answers 200 since the attempt was made; the reason is a named value because four reasons have no four status codes.
	t.Run("a refusal answers with which of the four it was", func(t *testing.T) {
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
				fixture := newTelegramDeliveryRouterUnderTest(t)
				expectStoredSettingOpened(fixture)
				fixture.messageDeliveryProxy.EXPECT().
					Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testCase.reason, nil)

				response := fixture.send(http.MethodPost,
					"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, true)

				require.Equal(t, http.StatusOK, response.Code)
				assert.Contains(t, response.Body.String(), `"delivered":false`)
				assert.Contains(t, response.Body.String(),
					`"failureReason":"`+testCase.expectedReason+`"`)
			})
		}
	})

	// 409 rather than 400: nothing typed is wrong, a setup step is missing.
	t.Run("without a setting the send is a conflict", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, true)

		require.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "尚未完成 Telegram 設定")
	})

	t.Run("a message that breaks a rule is a bad request", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"   "}`, true)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "訊息不得為空白")
	})

	t.Run("a token the system cannot open is the system being unavailable", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		fixture.telegramDeliveryRepository.EXPECT().
			FindOneByUser(gomock.Any(), signedInViewerID).
			Return(aStoredDeliveryFor(signedInViewerID), nil)
		fixture.secretSealProxy.EXPECT().
			Unseal(gomock.Any()).
			Return("", domains.ErrSecretSealUnavailable)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, true)

		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	})

	t.Run("a body that is not readable is a bad request", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{`, true)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("the sending side failing is reported as the system's problem", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)
		expectStoredSettingOpened(fixture)
		fixture.messageDeliveryProxy.EXPECT().
			Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(vo.DeliveryFailureUnreachable, errors.New("build message request failed"))

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, true)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})

	t.Run("without a proof of identity the handler never runs", func(t *testing.T) {
		fixture := newTelegramDeliveryRouterUnderTest(t)

		response := fixture.send(http.MethodPost,
			"/users/me/telegram-delivery/test-message", `{"message":"哈囉"}`, false)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})
}
