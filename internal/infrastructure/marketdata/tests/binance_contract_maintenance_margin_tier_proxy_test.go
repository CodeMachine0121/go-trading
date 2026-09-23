package marketdata_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testAccountKey    = "test-account-key"
	testAccountSecret = "test-account-secret"
)

// leverageBracketAnswer is the venue's answer as it spells it: bare JSON numbers,
// including a rate with more digits than a floating point value keeps exactly.
const leverageBracketAnswer = `[
 {"symbol":"BTCUSDT","notionalCoef":1.0,"brackets":[
   {"bracket":1,"initialLeverage":125,"notionalCap":50000,"notionalFloor":0,"maintMarginRatio":0.004,"cum":0.0},
   {"bracket":2,"initialLeverage":100,"notionalCap":250000,"notionalFloor":50000,"maintMarginRatio":0.005,"cum":50.0}]},
 {"symbol":"ETHUSDT","notionalCoef":1.0,"brackets":[
   {"bracket":1,"initialLeverage":100,"notionalCap":10000,"notionalFloor":0,"maintMarginRatio":0.00650000000000000011,"cum":0.0}]}
]`

var signedAt = time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)

func clockAt(t *testing.T, currentTime time.Time) *mocks.MockIClockProxy {
	t.Helper()

	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	return clockProxy
}

func TestMaintenanceMarginProxyReadsEveryContractsLadder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(leverageBracketAnswer))
	}))
	t.Cleanup(server.Close)
	tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
		server.URL, testAccountKey, testAccountSecret, requestTimeout, unpaced(), clockAt(t, signedAt))

	ladders, fetchError := tierProxy.FetchMaintenanceMarginLadders(t.Context())

	require.NoError(t, fetchError)
	require.Len(t, ladders, 2)
	assert.Equal(t, "BTCUSDT", ladders[0].Symbol)
	require.Len(t, ladders[0].Tiers, 2)
	second := ladders[0].Tiers[1]
	assert.Equal(t, 2, second.Tier)
	assert.Equal(t, 100, second.MaximumLeverage)
	assert.True(t, decimal.RequireFromString("50000").Equal(second.NotionalFloor))
	assert.True(t, decimal.RequireFromString("250000").Equal(second.NotionalCap))
	assert.True(t, decimal.RequireFromString("0.005").Equal(second.MaintenanceMarginRate))
	assert.True(t, decimal.RequireFromString("50").Equal(second.MaintenanceAmount))
	assert.True(t, decimal.RequireFromString("0.00650000000000000011").Equal(ladders[1].Tiers[0].MaintenanceMarginRate),
		"數字不該經過浮點數")
}

func TestMaintenanceMarginProxySignsTheQuestionAsTheAccount(t *testing.T) {
	var receivedKey, receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedKey = request.Header.Get("X-MBX-APIKEY")
		receivedQuery = request.URL.RawQuery
		_, _ = writer.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)
	tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
		server.URL, testAccountKey, testAccountSecret, requestTimeout, unpaced(), clockAt(t, signedAt))

	_, fetchError := tierProxy.FetchMaintenanceMarginLadders(t.Context())

	require.NoError(t, fetchError)
	assert.Equal(t, testAccountKey, receivedKey)
	unsigned, signature, hasSignature := strings.Cut(receivedQuery, "&signature=")
	require.True(t, hasSignature)
	assert.Contains(t, unsigned, "timestamp=1790150400000")
	assert.Contains(t, unsigned, "recvWindow=5000")
	signer := hmac.New(sha256.New, []byte(testAccountSecret))
	signer.Write([]byte(unsigned))
	assert.Equal(t, hex.EncodeToString(signer.Sum(nil)), signature)
	assert.NotContains(t, receivedQuery, testAccountSecret)
}

func TestMaintenanceMarginProxyAsksNothingWithoutAnAccount(t *testing.T) {
	asked := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		asked.Add(1)
	}))
	t.Cleanup(server.Close)

	for _, credentials := range [][2]string{{"", ""}, {testAccountKey, ""}, {"", testAccountSecret}} {
		tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
			server.URL, credentials[0], credentials[1], requestTimeout, unpaced(), clockAt(t, signedAt))

		ladders, fetchError := tierProxy.FetchMaintenanceMarginLadders(t.Context())

		assert.ErrorIs(t, fetchError, domains.ErrContractAccountCredentialsMissing)
		assert.Nil(t, ladders)
	}
	assert.Equal(t, int32(0), asked.Load())
}

func TestMaintenanceMarginProxyTellsARefusedAccountFromAnyOtherFailure(t *testing.T) {
	testCases := []struct {
		name             string
		statusCode       int
		body             string
		isRefusedAccount bool
	}{
		{name: "金鑰被拒 401", statusCode: http.StatusUnauthorized, body: `{"code":-2014,"msg":"API-key format invalid."}`, isRefusedAccount: true},
		{name: "權限不足 403", statusCode: http.StatusForbidden, isRefusedAccount: true},
		{name: "太多請求 429", statusCode: http.StatusTooManyRequests},
		{name: "讀不懂", statusCode: http.StatusOK, body: `{"not":"a list"}`},
		{name: "少了一個數字", statusCode: http.StatusOK,
			body: `[{"symbol":"BTCUSDT","brackets":[{"bracket":1,"initialLeverage":1,"notionalCap":50000,"notionalFloor":0,"maintMarginRatio":0.004}]}]`},
		{name: "數字讀不懂", statusCode: http.StatusOK,
			body: `[{"symbol":"BTCUSDT","brackets":[{"bracket":1,"initialLeverage":1,"notionalCap":"x","notionalFloor":0,"maintMarginRatio":0,"cum":0}]}]`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.statusCode)
				_, _ = writer.Write([]byte(testCase.body))
			}))
			t.Cleanup(server.Close)
			tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
				server.URL, testAccountKey, testAccountSecret, requestTimeout, unpaced(), clockAt(t, signedAt))

			ladders, fetchError := tierProxy.FetchMaintenanceMarginLadders(t.Context())

			require.Error(t, fetchError)
			assert.Nil(t, ladders)
			assert.Equal(t, testCase.isRefusedAccount, errors.Is(fetchError, domains.ErrContractAccountCredentialsRefused))
			assert.NotContains(t, fetchError.Error(), testAccountKey)
			assert.NotContains(t, fetchError.Error(), testAccountSecret)
		})
	}
}

func TestMaintenanceMarginProxySaysSoWhenTheVenueCannotBeReached(t *testing.T) {
	for _, address := range []string{"http://127.0.0.1:1", controlCharacterUrl} {
		tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
			address, testAccountKey, testAccountSecret, requestTimeout, unpaced(), clockAt(t, signedAt))

		_, fetchError := tierProxy.FetchMaintenanceMarginLadders(t.Context())

		assert.ErrorContains(t, fetchError, "reach contract account source")
		assert.NotContains(t, fetchError.Error(), testAccountKey)
	}
}

func TestMaintenanceMarginProxyGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	tierProxy := marketdata.NewBinanceContractMaintenanceMarginTierProxy(
		"http://127.0.0.1:1", testAccountKey, testAccountSecret, requestTimeout,
		marketdata.NewRequestPacer(1), clockAt(t, signedAt))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, fetchError := tierProxy.FetchMaintenanceMarginLadders(abandonedContext)

	assert.ErrorIs(t, fetchError, context.Canceled)
}
