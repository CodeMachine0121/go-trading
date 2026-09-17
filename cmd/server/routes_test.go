package main

import (
	"slices"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestMountedRoutesAreExactlyTheOnesIntended holds the whole reachable surface in
// one list, so that widening it is a decision somebody makes rather than a side
// effect somebody notices later.
//
// It used to hold a narrower boundary — that the watchlist could not be changed from
// outside at all. That boundary was deliberately given up: a watchlist that costs a
// restart to change is a watchlist nobody changes. What replaced it is narrower than
// it sounds: exactly two routes touch the watchlist, and neither can start a round,
// stop one, or reach the ingestion machinery itself.
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
		"DELETE /k-candles/:symbol/:openTime",
		// Taking a published strategy script off one's own shelf, and taking one's own
		// strategy script off the shared shelf. Two different withdrawals, so two paths:
		// one hangs off the marketplace, the other off the strategy script itself.
		"DELETE /marketplace/strategy-scripts/:id/adoption",
		"DELETE /strategy-bots/:id",
		"DELETE /strategy-bots/:id/power",
		"DELETE /strategy-scripts/:id",
		"DELETE /strategy-scripts/:id/publication",
		// 一份規則自己是一個東西，所以它有自己的一組路徑，而不是掛在機器人底下。
		"DELETE /trading-strategies/:id",
		// Taking away the place this system was told to speak to. It names nobody
		// but the person asking, so it cannot reach a symbol either.
		"DELETE /users/me/telegram-delivery",
		// Stopping and starting the watching of one market. Neither reaches ingestion
		// itself: a caller can say what to keep up to date, never when to do it.
		"DELETE /watchlist/:symbol",
		// The assistant reads and writes only through the very use cases a caller
		// already has; it is given no capability that touches the watchlist, so these
		// three do not widen what a caller can reach either.
		"GET /chat/conversations",
		"GET /chat/conversations/:id",
		"GET /health",
		"GET /k-candles",
		"GET /k-candles/:symbol/:openTime",
		// Following a market live reads; it names the symbol the viewer is looking at
		// and cannot touch the watchlist, so the boundary this test holds is intact.
		"GET /k-candles/live",
		"GET /k-candles/series",
		// The shared shelf. It hands out names, descriptions and knobs, never an
		// algorithm — that is a property of the shape it answers with, not of this
		// route.
		"GET /marketplace/strategy-scripts",
		"GET /strategy-bots",
		"GET /strategy-bots/:id",
		"GET /strategy-bots/:id/runs",
		"GET /strategy-scripts",
		"GET /strategy-scripts/:id",
		"GET /trading-strategies",
		"GET /trading-strategies/:id",
		"GET /trading-symbols",
		// Recognising a person reads and writes only users. None of these three can
		// name a symbol, so the boundary this test holds is intact.
		"GET /users/me",
		"GET /users/me/telegram-delivery",
		// Replaying a strategy script reads the market and stores nothing at all, so it
		// cannot reach the watchlist either.
		"POST /backtests",
		"POST /chat",
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
		// 重演掛在那一份底下，因為那是對它做的事。
		"POST /trading-strategies/:id/backtests",
		"POST /users",
		// Replacing one's own password. It names nobody but the person asking —
		// who that is comes from the proof on the request — so it cannot reach a
		// symbol or anybody else's account.
		"POST /users/me/password",
		// Sending one message to one's own Telegram. It reads the setting already
		// stored and writes nothing, so it widens nothing.
		"POST /users/me/telegram-delivery/test-message",
		"POST /watchlist",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /strategy-bots/:id",
		"PUT /strategy-scripts/:id",
		"PUT /trading-strategies/:id",
		"PUT /users/me/telegram-delivery",
	}, mountedRoutes)
}
