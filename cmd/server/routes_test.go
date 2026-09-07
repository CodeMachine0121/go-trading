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
		"DELETE /strategies/:id",
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
		"GET /strategies",
		"GET /strategies/:id",
		"GET /trading-symbols",
		// Recognising a person reads and writes only users. None of these three can
		// name a symbol, so the boundary this test holds is intact.
		"GET /users/me",
		// Replaying a strategy reads the market and stores nothing at all, so it
		// cannot reach the watchlist either.
		"POST /backtests",
		"POST /chat",
		"POST /indicator-calculations",
		"POST /k-candles",
		"POST /sessions",
		"POST /sessions/renewal",
		"POST /sessions/revocation",
		"POST /strategies",
		"POST /users",
		"POST /watchlist",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /strategies/:id",
	}, mountedRoutes)
}
