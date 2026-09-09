package main

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestBackgroundJobsForRespectsTheSwitch(t *testing.T) {
	testCases := []struct {
		name             string
		switchValue      string
		expectedJobCount int
	}{
		{name: "switched off leaves nothing to start", switchValue: "false", expectedJobCount: 0},
		{
			// Keeping the stored candles current, and handing out the live places of
			// markets that limit them. They are separate jobs so that a slow round
			// cannot hold up a market that has just opened.
			name:        "switched on assembles the work the system does on its own",
			switchValue: "true", expectedJobCount: 2,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("BACKGROUND_JOBS_ENABLED", testCase.switchValue)

			backgroundJobs := backgroundJobsFor(config.Load(), nil, nil)

			assert.Len(t, backgroundJobs, testCase.expectedJobCount)
		})
	}
}

// Every market the system recognises needs all three of its sources wired up, and
// this is the only place they are wired. A market with rules but no source reads as
// a market that is quietly holding nothing — which is how a missing wire-up survives
// to production.
//
// The addresses are pointed at nothing on purpose: what is being checked is that a
// source was reached for at all, not that it answered.
func TestEveryRecognisedMarketHasItsSourcesWiredUp(t *testing.T) {
	for _, urlVariable := range []string{
		"MARKET_DATA_BASE_URL", "MARKET_DATA_SYMBOL_CATALOG_URL",
		"TAIWAN_STOCK_INTRADAY_CANDLES_URL", "TAIWAN_STOCK_HISTORICAL_CANDLES_URL",
		"TAIWAN_STOCK_TICKER_URL",
		"TAIWAN_FUTURES_PRODUCTS_URL", "TAIWAN_FUTURES_INTRADAY_CANDLES_URL",
	} {
		t.Setenv(urlVariable, "http://127.0.0.1:1/nothing-is-listening")
	}

	applicationConfig := config.Load()
	marketDataProxy := marketDataProxyFor(applicationConfig)
	symbolLookupProxy := symbolLookupProxyFor(applicationConfig)
	liveMarketDataProxy := liveMarketDataProxyFor(applicationConfig)

	for market := range applicationConfig.MarketRules {
		t.Run(string(market), func(t *testing.T) {
			// "…is wired up for X" is how all three routing tables say a market has
			// no source. Any other error means one was reached and could not answer,
			// which is what these addresses were pointed at nothing to produce.
			_, fetchError := marketDataProxy.FetchKCandles(t.Context(),
				vo.NewKCandleFetchWindowVo("ANY", market, time.Now(), time.Now()))
			assert.NotContains(t, errorTextOf(fetchError), notWiredUp)

			_, lookUpError := symbolLookupProxy.LookUpSymbol(t.Context(), market, "ANY")
			assert.NotContains(t, errorTextOf(lookUpError), notWiredUp)

			_, followError := liveMarketDataProxy.FollowKCandles(t.Context(),
				vo.NewLiveFollowChannelVo(market, []string{"ANY"}))
			assert.NotContains(t, errorTextOf(followError), notWiredUp)
		})
	}
}

func errorTextOf(reportedError error) string {
	if reportedError == nil {
		return ""
	}

	return reportedError.Error()
}

// notWiredUp is what every routing table says when a market has no source at all.
const notWiredUp = "is wired up for"
