package exchange_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var aTradingKeyCredential = vo.TradingKeyCredentialVo{ApiKey: "the-api-key", SecretKey: "the-secret-key"}

var verificationMoment = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

func aClockAt(t *testing.T, moment time.Time) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
	clockProxy.EXPECT().Now().Return(moment).AnyTimes()

	return clockProxy
}

func binanceAnswering(t *testing.T, statusCode int, body string) *exchange.BinanceTradingKeyVerificationProxy {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(statusCode)
			_, _ = writer.Write([]byte(body))
		}))
	t.Cleanup(server.Close)

	return exchange.NewBinanceTradingKeyVerificationProxy(server.URL, server.Client(), aClockAt(t, verificationMoment))
}

func TestBinanceTradingKeyVerificationProxySendsASignedRequestForThePermissions(t *testing.T) {
	requestedPath := ""
	requestedQuery := ""
	sentApiKey := ""
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			requestedPath = request.URL.Path
			requestedQuery = request.URL.RawQuery
			sentApiKey = request.Header.Get("X-MBX-APIKEY")
			_, _ = writer.Write([]byte(`{"enableSpotAndMarginTrading":true,"enableFutures":false}`))
		}))
	t.Cleanup(server.Close)
	proxy := exchange.NewBinanceTradingKeyVerificationProxy(
		server.URL+"/", server.Client(), aClockAt(t, verificationMoment))

	_, err := proxy.VerifyTradingKey(t.Context(), aTradingKeyCredential)

	require.NoError(t, err)
	assert.Equal(t, "/sapi/v1/account/apiRestrictions", requestedPath)
	assert.Equal(t, "the-api-key", sentApiKey)
	unsignedQuery, signature, hasSignature := strings.Cut(requestedQuery, "&signature=")
	require.True(t, hasSignature)
	assert.Equal(t, "recvWindow=5000&timestamp=1790841600000", unsignedQuery)
	signer := hmac.New(sha256.New, []byte("the-secret-key"))
	signer.Write([]byte(unsignedQuery))
	assert.Equal(t, hex.EncodeToString(signer.Sum(nil)), signature)
	assert.NotContains(t, requestedQuery, "the-secret-key")
}

func TestBinanceTradingKeyVerificationProxyReadsTheTradingPermissions(t *testing.T) {
	testCases := []struct {
		name             string
		body             string
		expectedSpot     bool
		expectedContract bool
	}{
		{name: "both", body: `{"enableReading":true,"enableSpotAndMarginTrading":true,"enableFutures":true,"enableWithdrawals":true}`, expectedSpot: true, expectedContract: true},
		{name: "contract only", body: `{"enableSpotAndMarginTrading":false,"enableFutures":true}`, expectedContract: true},
		{name: "read only", body: `{"enableReading":true,"enableSpotAndMarginTrading":false,"enableFutures":false}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verification, err := binanceAnswering(t, http.StatusOK, testCase.body).
				VerifyTradingKey(t.Context(), aTradingKeyCredential)

			require.NoError(t, err)
			assert.Equal(t, vo.TradingKeyVerificationFailureNone, verification.FailureReason)
			assert.Equal(t, testCase.expectedSpot, verification.SpotTradingEnabled)
			assert.Equal(t, testCase.expectedContract, verification.ContractTradingEnabled)
		})
	}
}

func TestBinanceTradingKeyVerificationProxySortsEveryRefusal(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		body           string
		expectedReason vo.TradingKeyVerificationFailureVo
	}{
		{name: "an unknown key", statusCode: http.StatusUnauthorized, body: `{"code":-2015,"msg":"Invalid API-key, IP, or permissions for action."}`, expectedReason: vo.TradingKeyVerificationFailureKeyRejected},
		{name: "a malformed key", statusCode: http.StatusBadRequest, body: `{"code":-2014,"msg":"API-key format invalid."}`, expectedReason: vo.TradingKeyVerificationFailureKeyRejected},
		{name: "a wrong secret key", statusCode: http.StatusBadRequest, body: `{"code":-1022,"msg":"Signature for this request is not valid."}`, expectedReason: vo.TradingKeyVerificationFailureKeyRejected},
		{name: "an unauthorized answer without a body", statusCode: http.StatusUnauthorized, body: ``, expectedReason: vo.TradingKeyVerificationFailureKeyRejected},
		// Our clock being off is not the key's fault, so nobody is sent to regenerate it.
		{name: "a timestamp outside the window", statusCode: http.StatusBadRequest, body: `{"code":-1021,"msg":"Timestamp for this request is outside of the recvWindow."}`, expectedReason: vo.TradingKeyVerificationFailureUnreachable},
		{name: "being rate limited", statusCode: http.StatusTooManyRequests, body: `{"code":-1003,"msg":"Too many requests"}`, expectedReason: vo.TradingKeyVerificationFailureUnreachable},
		{name: "a server failure", statusCode: http.StatusInternalServerError, body: `oops`, expectedReason: vo.TradingKeyVerificationFailureUnreachable},
		{name: "an unreadable success", statusCode: http.StatusOK, body: `<html>`, expectedReason: vo.TradingKeyVerificationFailureUnreachable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verification, err := binanceAnswering(t, testCase.statusCode, testCase.body).
				VerifyTradingKey(t.Context(), aTradingKeyCredential)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedReason, verification.FailureReason)
		})
	}
}

func TestBinanceTradingKeyVerificationProxyReportsUnreachableBinance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachableAddress := server.URL
	server.Close()

	verification, err := exchange.NewBinanceTradingKeyVerificationProxy(
		unreachableAddress, http.DefaultClient, aClockAt(t, verificationMoment),
	).VerifyTradingKey(t.Context(), aTradingKeyCredential)

	require.NoError(t, err)
	assert.Equal(t, vo.TradingKeyVerificationFailureUnreachable, verification.FailureReason)
}

func TestBinanceTradingKeyVerificationProxyReportsAnUnusableAddressAsUnreachable(t *testing.T) {
	verification, err := exchange.NewBinanceTradingKeyVerificationProxy(
		"http://bad host", http.DefaultClient, aClockAt(t, verificationMoment),
	).VerifyTradingKey(t.Context(), aTradingKeyCredential)

	require.NoError(t, err)
	assert.Equal(t, vo.TradingKeyVerificationFailureUnreachable, verification.FailureReason)
}

func TestBinanceTradingKeyVerificationProxyReportsASlowBinanceAsTimedOut(t *testing.T) {
	testCases := []struct {
		name          string
		answerHeaders bool
	}{
		{name: "no answer at all within the wait"},
		{name: "an answer that stops halfway", answerHeaders: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, request *http.Request) {
					if testCase.answerHeaders {
						writer.WriteHeader(http.StatusOK)
						_, _ = writer.Write([]byte(`{"enableSpotAndMarginTrading":`))
						writer.(http.Flusher).Flush()
					}
					<-release
				}))
			t.Cleanup(server.Close)
			t.Cleanup(func() { close(release) })

			verification, err := exchange.NewBinanceTradingKeyVerificationProxy(
				server.URL, &http.Client{Timeout: 100 * time.Millisecond}, aClockAt(t, verificationMoment),
			).VerifyTradingKey(t.Context(), aTradingKeyCredential)

			require.NoError(t, err)
			assert.Equal(t, vo.TradingKeyVerificationFailureTimedOut, verification.FailureReason)
		})
	}
}
