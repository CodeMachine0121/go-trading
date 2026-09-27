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
		"DELETE /contract-trade-records/:id",
		"DELETE /contract-trade-records/:id/fills/:fillId",
		"DELETE /contract-watchlist/:symbol",
		"DELETE /k-candles/:symbol/:openTime",
		"DELETE /spot-trade-records/:id",
		"DELETE /spot-trade-records/:id/fills/:fillId",
		"DELETE /strategy-bots/:id",
		"DELETE /strategy-bots/:id/power",
		"DELETE /strategy-scripts/:id",
		"DELETE /strategy-scripts/:id/publication",
		"DELETE /trading-strategies/:id",
		"DELETE /users/me/telegram-delivery",
		"DELETE /users/me/trade-tags/:id",
		"DELETE /watchlist/:symbol",
		"GET /.well-known/oauth-authorization-server",
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
		"GET /contract-trade-records",
		"GET /contract-trade-records/:id",
		"GET /contract-trade-records/journal-links/:identifier",
		"GET /contract-trade-records/statistics",
		"GET /contract-trading-symbols",
		"GET /health",
		"GET /k-candles",
		"GET /k-candles/:symbol/:openTime",
		"GET /k-candles/history/:id",
		"GET /k-candles/live",
		"GET /k-candles/series",
		"GET /marketplace/strategy-scripts",
		"GET /oauth/authorization-requests/:requestId",
		"GET /oauth/authorize",
		"GET /spot-trade-records",
		"GET /spot-trade-records/:id",
		"GET /spot-trade-records/journal-links/:identifier",
		"GET /spot-trade-records/statistics",
		"GET /strategy-bots",
		"GET /strategy-bots/:id",
		"GET /strategy-bots/:id/runs",
		"GET /strategy-scripts",
		"GET /strategy-scripts/:id",
		"GET /trading-strategies",
		"GET /trading-strategies/:id",
		"GET /trading-strategies/:id/contract-trade-comparison",
		"GET /trading-strategies/:id/spot-trade-comparison",
		"GET /trading-symbols",
		"GET /users/me",
		"GET /users/me/telegram-delivery",
		"GET /users/me/trade-journal-settings",
		"GET /users/me/trade-tags",
		"POST /backtests",
		"POST /chat",
		"POST /chat/pending-revisions/:id/confirm",
		"POST /chat/pending-revisions/:id/reject",
		"POST /contract-backtests",
		"POST /contract-indicator-calculations",
		"POST /contract-k-candles",
		"POST /contract-k-candles/backfill",
		"POST /contract-k-candles/history",
		"POST /contract-trade-records",
		"POST /contract-trade-records/:id/fills",
		"POST /contract-trade-records/:id/notes",
		"POST /contract-watchlist",
		"POST /indicator-calculations",
		"POST /k-candles",
		"POST /k-candles/backfill",
		"POST /k-candles/history",
		"POST /marketplace/strategy-scripts/:id/adoption",
		"POST /oauth/authorization-requests/:requestId/approval",
		"POST /oauth/authorization-requests/:requestId/denial",
		"POST /oauth/introspection",
		"POST /oauth/register",
		"POST /oauth/token",
		"POST /sessions",
		"POST /sessions/renewal",
		"POST /sessions/revocation",
		"POST /spot-trade-records",
		"POST /spot-trade-records/:id/fills",
		"POST /spot-trade-records/:id/notes",
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
		"POST /users/me/trade-tags",
		"POST /watchlist",
		"PUT /contract-k-candles/:symbol/:openTime",
		"PUT /contract-trade-records/:id/fills/:fillId",
		"PUT /contract-trade-records/:id/plan",
		"PUT /contract-trade-records/:id/review",
		"PUT /contract-trade-records/:id/setup-tags",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /spot-trade-records/:id/fills/:fillId",
		"PUT /spot-trade-records/:id/plan",
		"PUT /spot-trade-records/:id/review",
		"PUT /spot-trade-records/:id/setup-tags",
		"PUT /strategy-bots/:id",
		"PUT /strategy-scripts/:id",
		"PUT /trading-strategies/:id",
		"PUT /users/me/telegram-delivery",
		"PUT /users/me/trade-journal-settings",
		"PUT /users/me/trade-tags/:id",
	}, mountedRoutes)
}
