package controller_test

import (
	"context"
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

type binanceTradingKeyRouterUnderTest struct {
	engine                      *gin.Engine
	binanceTradingKeyRepository *mocks.MockIBinanceTradingKeyRepository
	secretSealProxy             *mocks.MockISecretSealProxy
	tradingKeyVerificationProxy *mocks.MockITradingKeyVerificationProxy
}

func newBinanceTradingKeyRouterUnderTest(t *testing.T) binanceTradingKeyRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	binanceTradingKeyRepository := mocks.NewMockIBinanceTradingKeyRepository(mockController)
	secretSealProxy := mocks.NewMockISecretSealProxy(mockController)
	tradingKeyVerificationProxy := mocks.NewMockITradingKeyVerificationProxy(mockController)

	binanceTradingKeyController := controller.NewBinanceTradingKeyController(
		application.NewBinanceTradingKeyApplication(
			service.NewBinanceTradingKeyService(
				binanceTradingKeyRepository, secretSealProxy, tradingKeyVerificationProxy)))

	requiresSignIn := doorOpenFor(t, signedInViewerID)

	engine := gin.New()
	engine.GET("/users/me/binance-trading-key", requiresSignIn, binanceTradingKeyController.GetTradingKey)
	engine.GET("/users/me/binance-trading-key/status", requiresSignIn, binanceTradingKeyController.GetTradingKeyStatus)
	engine.PUT("/users/me/binance-trading-key", requiresSignIn, binanceTradingKeyController.SaveTradingKey)
	engine.DELETE("/users/me/binance-trading-key", requiresSignIn, binanceTradingKeyController.RemoveTradingKey)

	return binanceTradingKeyRouterUnderTest{
		engine:                      engine,
		binanceTradingKeyRepository: binanceTradingKeyRepository,
		secretSealProxy:             secretSealProxy,
		tradingKeyVerificationProxy: tradingKeyVerificationProxy,
	}
}

func (fixture binanceTradingKeyRouterUnderTest) send(
	method string, path string, body string, signedIn bool,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if signedIn {
		request.Header.Set("Authorization", signedInProof)
	}
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func (fixture binanceTradingKeyRouterUnderTest) expectSealing() {
	fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).
		DoAndReturn(func(plaintext string) (string, error) { return "sealed-" + plaintext, nil }).AnyTimes()
}

const aTradingKeyBody = `{"apiKey":"the-api-key-a1b2","secretKey":"the-secret-key"}`

func aStoredBinanceTradingKeyFor(userID uint) entities.BinanceTradingKey {
	return entities.BinanceTradingKey{
		ID: 1, UserID: userID, SealedApiKey: "the-sealed-api-key", SealedSecretKey: "the-sealed-secret-key",
		ApiKeyTail: "a1b2", SpotTradingEnabled: true,
		UpdatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}
}

func TestBinanceTradingKeyRouterReads(t *testing.T) {
	t.Run("the owner reads the tail, markets and time but no sealed string", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
			Return(aStoredBinanceTradingKeyFor(signedInViewerID), nil)

		response := fixture.send(http.MethodGet, "/users/me/binance-trading-key", "", true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t,
			`{"configured":true,"apiKeyTail":"a1b2","tradableMarkets":["spot"],"configuredAt":"2026-10-01T08:00:00Z"}`,
			response.Body.String())
	})

	t.Run("the status carries no part of the key", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
			Return(aStoredBinanceTradingKeyFor(signedInViewerID), nil)

		response := fixture.send(http.MethodGet, "/users/me/binance-trading-key/status", "", true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t,
			`{"configured":true,"tradableMarkets":["spot"],"configuredAt":"2026-10-01T08:00:00Z"}`,
			response.Body.String())
	})

	t.Run("never having stored one answers 200 saying so", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured)

		response := fixture.send(http.MethodGet, "/users/me/binance-trading-key", "", true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"configured":false`)
		assert.Contains(t, response.Body.String(), `"tradableMarkets":[]`)
	})

	t.Run("storage failing is the system's problem on both reads", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.BinanceTradingKey{}, errors.New("the database is not there")).Times(2)

		assert.Equal(t, http.StatusBadGateway,
			fixture.send(http.MethodGet, "/users/me/binance-trading-key", "", true).Code)
		assert.Equal(t, http.StatusBadGateway,
			fixture.send(http.MethodGet, "/users/me/binance-trading-key/status", "", true).Code)
	})

	t.Run("without a proof of identity nothing is read", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)

		assert.Equal(t, http.StatusUnauthorized,
			fixture.send(http.MethodGet, "/users/me/binance-trading-key", "", false).Code)
		assert.Equal(t, http.StatusUnauthorized,
			fixture.send(http.MethodGet, "/users/me/binance-trading-key/status", "", false).Code)
	})
}

func TestBinanceTradingKeyRouterSaveTradingKey(t *testing.T) {
	t.Run("an accepted key answers with its tail and never the secret key", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.expectSealing()
		fixture.tradingKeyVerificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), gomock.Any()).
			Return(vo.TradingKeyVerificationVo{SpotTradingEnabled: true, ContractTradingEnabled: true}, nil)
		fixture.binanceTradingKeyRepository.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, binanceTradingKey entities.BinanceTradingKey, _ []string) (entities.BinanceTradingKey, error) {
				return binanceTradingKey, nil
			})

		response := fixture.send(http.MethodPut, "/users/me/binance-trading-key", aTradingKeyBody, true)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"apiKeyTail":"a1b2"`)
		assert.Contains(t, response.Body.String(), `"tradableMarkets":["spot","contract"]`)
		assert.NotContains(t, response.Body.String(), "the-secret-key")
		assert.NotContains(t, response.Body.String(), "the-api-key-a1b2")
	})

	t.Run("a blank string is a bad request", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/binance-trading-key",
			`{"apiKey":"  ","secretKey":"the-secret-key"}`, true)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "必須給 API Key")
	})

	t.Run("an unreadable body is a bad request", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/binance-trading-key", `{"apiKey":`, true)

		require.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("a missing sealing key is the system's problem", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).Return("", domains.ErrSecretSealUnavailable).AnyTimes()

		response := fixture.send(http.MethodPut, "/users/me/binance-trading-key", aTradingKeyBody, true)

		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Contains(t, response.Body.String(), "系統目前無法安全保存幣安交易金鑰")
	})

	t.Run("each Binance refusal has its own status and named reason", func(t *testing.T) {
		testCases := []struct {
			name           string
			verification   vo.TradingKeyVerificationVo
			expectedStatus int
			expectedReason string
		}{
			{name: "key rejected", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, expectedStatus: http.StatusUnprocessableEntity, expectedReason: "keyRejected"},
			{name: "no trading permission", verification: vo.TradingKeyVerificationVo{}, expectedStatus: http.StatusUnprocessableEntity, expectedReason: "noTradingPermission"},
			{name: "unreachable", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, expectedStatus: http.StatusBadGateway, expectedReason: "unreachable"},
			{name: "timed out", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut}, expectedStatus: http.StatusGatewayTimeout, expectedReason: "timedOut"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newBinanceTradingKeyRouterUnderTest(t)
				fixture.expectSealing()
				fixture.tradingKeyVerificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), gomock.Any()).
					Return(testCase.verification, nil)

				response := fixture.send(http.MethodPut, "/users/me/binance-trading-key", aTradingKeyBody, true)

				require.Equal(t, testCase.expectedStatus, response.Code)
				assert.Contains(t, response.Body.String(), `"failureReason":"`+testCase.expectedReason+`"`)
				assert.NotContains(t, response.Body.String(), "the-secret-key")
			})
		}
	})

	t.Run("without a proof of identity nothing is stored", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/users/me/binance-trading-key", aTradingKeyBody, false)

		require.Equal(t, http.StatusUnauthorized, response.Code)
	})
}

func TestBinanceTradingKeyRouterRemoveTradingKey(t *testing.T) {
	t.Run("removing answers 204 whether or not a key existed", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().DeleteByUser(gomock.Any(), signedInViewerID).Return(nil)

		response := fixture.send(http.MethodDelete, "/users/me/binance-trading-key", "", true)

		require.Equal(t, http.StatusNoContent, response.Code)
	})

	t.Run("a store failure is the system's problem", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().DeleteByUser(gomock.Any(), signedInViewerID).
			Return(errors.New("the database is not there"))

		response := fixture.send(http.MethodDelete, "/users/me/binance-trading-key", "", true)

		require.Equal(t, http.StatusBadGateway, response.Code)
	})

	t.Run("without a proof of identity nothing is removed", func(t *testing.T) {
		fixture := newBinanceTradingKeyRouterUnderTest(t)

		require.Equal(t, http.StatusUnauthorized,
			fixture.send(http.MethodDelete, "/users/me/binance-trading-key", "", false).Code)
	})
}
