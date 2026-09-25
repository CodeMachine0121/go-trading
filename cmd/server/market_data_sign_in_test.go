package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSigningKey = "market-data-sign-in-test-key"

// newMountedEngine registers the real routes without a database; Recovery turns a handler touching it into a 500.
func newMountedEngine(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	t.Setenv("BACKGROUND_JOBS_ENABLED", "false")
	t.Setenv("AUTH_ACCESS_TOKEN_SIGNING_KEY", testSigningKey)
	engine := gin.New()
	engine.Use(gin.Recovery())

	registerRoutes(engine, nil, config.Load())

	return engine
}

func expiredAccessToken(t *testing.T) string {
	accessToken, issueError := security.NewJwtAccessTokenProxy(testSigningKey).
		Issue(1, time.Now().Add(-time.Minute))
	require.NoError(t, issueError)

	return "Bearer " + accessToken.AccessToken
}

func requestMounted(engine *gin.Engine, method string, target string, authorization string) int {
	request := httptest.NewRequest(method, target, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	return recorder.Code
}

func TestChangingMarketDataRequiresSignIn(t *testing.T) {
	engine := newMountedEngine(t)
	expiredProof := expiredAccessToken(t)

	marketDataChanges := []struct {
		method string
		target string
	}{
		{method: http.MethodPost, target: "/k-candles"},
		{method: http.MethodPut, target: "/k-candles/BTCUSDT/2026-08-29T09:00:00Z"},
		{method: http.MethodDelete, target: "/k-candles/BTCUSDT/2026-08-29T09:00:00Z"},
		{method: http.MethodPost, target: "/k-candles/backfill"},
		{method: http.MethodPost, target: "/k-candles/history"},
		{method: http.MethodGet, target: "/k-candles/history/1"},
		{method: http.MethodPost, target: "/watchlist"},
		{method: http.MethodDelete, target: "/watchlist/ETHUSDT"},
		{method: http.MethodPost, target: "/contract-k-candles"},
		{method: http.MethodPut, target: "/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z"},
		{method: http.MethodDelete, target: "/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z"},
		{method: http.MethodPost, target: "/contract-k-candles/backfill"},
		{method: http.MethodPost, target: "/contract-k-candles/history"},
		{method: http.MethodGet, target: "/contract-k-candles/history/1"},
		{method: http.MethodPost, target: "/contract-watchlist"},
		{method: http.MethodDelete, target: "/contract-watchlist/BTCUSDT"},
	}

	for _, change := range marketDataChanges {
		t.Run(change.method+" "+change.target, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized,
				requestMounted(engine, change.method, change.target, ""), "without a proof")
			assert.Equal(t, http.StatusUnauthorized,
				requestMounted(engine, change.method, change.target, expiredProof), "with an expired proof")
		})
	}
}

func TestReadingMarketDataStaysPublic(t *testing.T) {
	engine := newMountedEngine(t)
	expiredProof := expiredAccessToken(t)

	marketDataReads := []string{
		"/k-candles",
		"/k-candles/series",
		"/k-candles/BTCUSDT/2026-08-29T09:00:00Z",
		"/k-candles/live",
		"/trading-symbols",
		"/contract-k-candles",
		"/contract-k-candles/series",
		"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z",
		"/contract-k-candles/live",
		"/contract-trading-symbols",
		"/contract-funding-rate-settlements",
		"/contract-maintenance-margin-tiers",
		"/contract-position-statistics",
	}

	for _, target := range marketDataReads {
		t.Run(target, func(t *testing.T) {
			assert.NotEqual(t, http.StatusUnauthorized,
				requestMounted(engine, http.MethodGet, target, ""), "without a proof")
			assert.NotEqual(t, http.StatusUnauthorized,
				requestMounted(engine, http.MethodGet, target, expiredProof), "with an expired proof")
		})
	}
}
