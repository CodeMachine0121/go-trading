package main

import (
	"slices"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestMountedRoutesAreExactlyTheOnesIntended pins the whole reachable surface so widening it is a
// deliberate decision; no route may start or stop an ingestion round.
func TestMountedRoutesAreExactlyTheOnesIntended(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("BACKGROUND_JOBS_ENABLED", "true")
	engine := gin.New()

	registerRoutes(engine, nil, config.Load())

	mountedRoutes := make([]string, 0)
	for _, route := range engine.Routes() {
		mountedRoutes = append(mountedRoutes, route.Method+" "+route.Path)
	}
	slices.Sort(mountedRoutes)

	assert.Equal(t, []string{
		"DELETE /contract-k-candles/:symbol/:openTime",
		"DELETE /contract-watchlist/:symbol",
		"DELETE /k-candles/:symbol/:openTime",
		"DELETE /marketplace/strategy-scripts/:id/adoption",
		"DELETE /strategy-bots/:id",
		"DELETE /strategy-bots/:id/power",
		"DELETE /strategy-scripts/:id",
		"DELETE /strategy-scripts/:id/publication",
		"DELETE /trading-strategies/:id",
		"DELETE /users/me/telegram-delivery",
		// Callers choose what to keep up to date, never when ingestion runs.
		"DELETE /watchlist/:symbol",
		"GET /chat/conversations",
		"GET /chat/conversations/:id",
		"GET /contract-funding-rate-settlements",
		"GET /contract-k-candles",
		"GET /contract-k-candles/:symbol/:openTime",
		"GET /contract-k-candles/history/:id",
		"GET /contract-k-candles/live",
		"GET /contract-k-candles/series",
		"GET /contract-maintenance-margin-tiers",
		"GET /contract-position-statistics",
		"GET /contract-trading-symbols",
		"GET /health",
		"GET /k-candles",
		"GET /k-candles/:symbol/:openTime",
		"GET /k-candles/history/:id",
		"GET /k-candles/live",
		"GET /k-candles/series",
		// Hands out names, descriptions and knobs, never the algorithm itself.
		"GET /marketplace/strategy-scripts",
		"GET /strategy-bots",
		"GET /strategy-bots/:id",
		"GET /strategy-bots/:id/runs",
		"GET /strategy-scripts",
		"GET /strategy-scripts/:id",
		"GET /trading-strategies",
		"GET /trading-strategies/:id",
		"GET /trading-symbols",
		"GET /users/me",
		"GET /users/me/telegram-delivery",
		"POST /backtests",
		"POST /chat",
		// Confirming carries out the assistant's proposed rewrite; there is no route that proposes one.
		"POST /chat/pending-revisions/:id/confirm",
		"POST /chat/pending-revisions/:id/reject",
		"POST /contract-backtests",
		"POST /contract-indicator-calculations",
		"POST /contract-k-candles",
		"POST /contract-k-candles/backfill",
		"POST /contract-k-candles/history",
		"POST /contract-watchlist",
		"POST /indicator-calculations",
		"POST /k-candles",
		"POST /k-candles/backfill",
		"POST /k-candles/history",
		"POST /marketplace/strategy-scripts/:id/adoption",
		"POST /sessions",
		"POST /sessions/renewal",
		"POST /sessions/revocation",
		"POST /strategy-bots",
		"POST /strategy-bots/:id/power",
		"POST /strategy-bots/:id/runs",
		"POST /strategy-scripts",
		"POST /strategy-scripts/:id/publication",
		"POST /trading-strategies",
		"POST /trading-strategies/:id/backtests",
		"POST /trading-strategies/:id/contract-backtests",
		"POST /users",
		"POST /users/me/password",
		"POST /users/me/telegram-delivery/test-message",
		"POST /watchlist",
		"PUT /contract-k-candles/:symbol/:openTime",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /strategy-bots/:id",
		"PUT /strategy-scripts/:id",
		"PUT /trading-strategies/:id",
		"PUT /users/me/telegram-delivery",
	}, mountedRoutes)
}
