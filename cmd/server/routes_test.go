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
		"GET /health",
		"GET /k-candles",
		"GET /k-candles/:symbol/:openTime",
		"GET /k-candles/series",
		"GET /strategies",
		"GET /strategies/:id",
		"GET /trading-symbols",
		"POST /indicator-calculations",
		"POST /k-candles",
		"POST /strategies",
		"PUT /k-candles/:symbol/:openTime",
		"PUT /strategies/:id",
	}, mountedRoutes)
}
