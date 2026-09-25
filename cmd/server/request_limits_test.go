package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAddressIsBelievedOnlyFromTrustedProxies(t *testing.T) {
	testCases := []struct {
		name            string
		trustedProxies  string
		clientIpHeaders string
		directPeer      string
		forwardedHeader string
		forwardedValue  string
		expectedAddress string
	}{
		{
			name:            "with no trusted proxy a forwarded address is ignored",
			directPeer:      "192.0.2.1:40000",
			forwardedHeader: "X-Forwarded-For",
			forwardedValue:  "1.2.3.4",
			expectedAddress: "192.0.2.1",
		},
		{
			name:            "the edge's address is believed when relayed by the ingress",
			trustedProxies:  "10.42.0.0/16",
			clientIpHeaders: "CF-Connecting-IP,X-Forwarded-For",
			directPeer:      "10.42.0.9:40000",
			forwardedHeader: "CF-Connecting-IP",
			forwardedValue:  "1.2.3.4",
			expectedAddress: "1.2.3.4",
		},
		{
			name:            "the same claim from outside the trusted range is ignored",
			trustedProxies:  "10.42.0.0/16",
			clientIpHeaders: "CF-Connecting-IP,X-Forwarded-For",
			directPeer:      "192.0.2.1:40000",
			forwardedHeader: "CF-Connecting-IP",
			forwardedValue:  "1.2.3.4",
			expectedAddress: "192.0.2.1",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			t.Setenv("TRUSTED_PROXY_CIDRS", testCase.trustedProxies)
			t.Setenv("CLIENT_IP_HEADERS", testCase.clientIpHeaders)
			engine := gin.New()
			guardRequests(engine, config.Load())
			engine.GET("/client-address", func(ginContext *gin.Context) {
				ginContext.String(http.StatusOK, ginContext.ClientIP())
			})
			request := httptest.NewRequest(http.MethodGet, "/client-address", nil)
			request.RemoteAddr = testCase.directPeer
			request.Header.Set(testCase.forwardedHeader, testCase.forwardedValue)

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, testCase.expectedAddress, recorder.Body.String())
		})
	}
}

func TestCredentialRoutesHaveTheStricterAllowance(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		clientAddress string
	}{
		{name: "registration", path: "/users", clientAddress: "198.51.100.1:40000"},
		{name: "sign-in", path: "/sessions", clientAddress: "198.51.100.2:40000"},
		{name: "session renewal", path: "/sessions/renewal", clientAddress: "198.51.100.3:40000"},
		{name: "password change", path: "/users/me/password", clientAddress: "198.51.100.4:40000"},
	}
	gin.SetMode(gin.TestMode)
	// One engine for every case, each from its own address, because building the routes is slow.
	engine := gin.New()
	registerRoutes(engine, nil, config.Load())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			// Malformed bodies are refused before any storage is touched, yet still spend the allowance.
			recorders := make([]*httptest.ResponseRecorder, 0, 11)
			for range 11 {
				request := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader("{"))
				request.RemoteAddr = testCase.clientAddress
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				recorders = append(recorders, recorder)
			}

			for _, admitted := range recorders[:10] {
				assert.NotEqual(t, http.StatusTooManyRequests, admitted.Code)
			}
			assert.Equal(t, http.StatusTooManyRequests, recorders[10].Code)
			assert.Equal(t, "6", recorders[10].Header().Get("Retry-After"))
		})
	}
}

func TestPreflightRequestsSpendNoAllowance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerRoutes(engine, nil, config.Load())

	for range 130 {
		preflight := httptest.NewRequest(http.MethodOptions, "/k-candles", nil)
		preflight.Header.Set("Origin", "http://localhost:3000")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, preflight)
		assert.Equal(t, http.StatusNoContent, recorder.Code)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestLiveRoutesHoldALiveStreamPlace(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{name: "spot", path: "/k-candles/live"},
		{name: "contract", path: "/contract-k-candles/live"},
	}
	gin.SetMode(gin.TestMode)
	handlerNames := make([]string, 0)
	engine := gin.New()
	engine.Use(func(ginContext *gin.Context) { handlerNames = ginContext.HandlerNames() })
	registerRoutes(engine, nil, config.Load())

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// No symbol, so the follow is refused before any market is touched.
			engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, testCase.path, nil))

			assert.True(t, slices.ContainsFunc(handlerNames, func(handlerName string) bool {
				return strings.Contains(handlerName, "LiveStreamLimitMiddleware")
			}), "the live stream cap is not mounted on %s", testCase.path)
		})
	}
}

// TestInvalidTrustedProxiesStopStartup re-runs itself as a child, since the refusal ends the process.
func TestInvalidTrustedProxiesStopStartup(t *testing.T) {
	if os.Getenv("GUARD_REQUESTS_CHILD") == "1" {
		guardRequests(gin.New(), config.Load())
		return
	}

	child := exec.Command(os.Args[0], "-test.run=^TestInvalidTrustedProxiesStopStartup$")
	child.Env = append(os.Environ(), "GUARD_REQUESTS_CHILD=1", "TRUSTED_PROXY_CIDRS=not-a-range")
	runError := child.Run()

	var exitError *exec.ExitError
	require.ErrorAs(t, runError, &exitError)
	assert.NotEqual(t, 0, exitError.ExitCode())
}
