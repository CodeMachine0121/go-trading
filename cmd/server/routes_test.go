package main

import (
	"slices"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestAutomaticIngestionOpensNoWayIn holds the boundary that automatic ingestion is
// something the system does to itself: it must never become something a caller can
// reach, because the watchlist would then be changeable from outside.
func TestAutomaticIngestionOpensNoWayIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("BACKGROUND_JOBS_ENABLED", "true")
	t.Setenv("KCANDLE_INGESTION_SYMBOLS", "BTCUSDT,ETHUSDT")
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
		"POST /chat",
		"POST /indicator-calculations",
		"POST /k-candles",
		"POST /sessions",
		"POST /strategies",
		"POST /users",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /strategies/:id",
	}, mountedRoutes)
}
