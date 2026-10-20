package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Saving a whole board can erase what the person drew by hand, so a connector's sign-in counts for nothing there.
func TestSavingAWholeChartDrawingBoardRefusesConnectors(t *testing.T) {
	engine := newMountedEngine(t)

	assert.Equal(t, http.StatusUnauthorized,
		requestMounted(engine, http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT", connectorAccessToken(t)))
	assert.Equal(t, http.StatusUnauthorized,
		requestMounted(engine, http.MethodPut, "/users/me/chart-drawings/spot/BTCUSDT",
			audiencelessConnectorAccessToken(t)))
}

// A connector reads the board and draws or removes only the assistant's own drawings.
func TestConnectorsReachTheAssistantDrawingRoutes(t *testing.T) {
	engine := newMountedEngine(t)
	connectorProof := connectorAccessToken(t)

	connectorRoutes := []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/users/me/chart-drawings/spot/BTCUSDT"},
		{method: http.MethodPost, target: "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings"},
		{method: http.MethodDelete, target: "/users/me/chart-drawings/spot/BTCUSDT/assistant-drawings"},
	}

	for _, route := range connectorRoutes {
		t.Run(route.method+" "+route.target, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, requestMounted(engine, route.method, route.target, ""))
			assert.NotEqual(t, http.StatusUnauthorized,
				requestMounted(engine, route.method, route.target, connectorProof))
		})
	}
}
