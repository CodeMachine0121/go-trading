package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A trading key must never pass through an AI conversation, so a connector's sign-in counts for nothing on these routes.
func TestBinanceTradingKeyWritesAndKeyTailRefuseConnectors(t *testing.T) {
	engine := newMountedEngine(t)
	connectorProof := connectorAccessToken(t)
	audiencelessConnectorProof := audiencelessConnectorAccessToken(t)

	webOnlyRoutes := []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/users/me/binance-trading-key"},
		{method: http.MethodPut, target: "/users/me/binance-trading-key"},
		{method: http.MethodDelete, target: "/users/me/binance-trading-key"},
		{method: http.MethodPost, target: "/strategy-bots/1/auto-order"},
		{method: http.MethodDelete, target: "/strategy-bots/1/auto-order"},
		{method: http.MethodPost, target: "/oauth/authorization-requests/request-1/approval"},
	}

	for _, route := range webOnlyRoutes {
		t.Run(route.method+" "+route.target, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized,
				requestMounted(engine, route.method, route.target, ""), "without a proof")
			assert.Equal(t, http.StatusUnauthorized,
				requestMounted(engine, route.method, route.target, connectorProof), "with a connector's token")
			assert.Equal(t, http.StatusUnauthorized,
				requestMounted(engine, route.method, route.target, audiencelessConnectorProof),
				"with a connector's token that names no audience")
		})
	}
}

func TestBinanceTradingKeyStatusIsReadableByConnectors(t *testing.T) {
	engine := newMountedEngine(t)

	assert.Equal(t, http.StatusUnauthorized,
		requestMounted(engine, http.MethodGet, "/users/me/binance-trading-key/status", ""))
	assert.NotEqual(t, http.StatusUnauthorized,
		requestMounted(engine, http.MethodGet, "/users/me/binance-trading-key/status", connectorAccessToken(t)))
}
