package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestTradableMarketsDomainCovers(t *testing.T) {
	testCases := []struct {
		name             string
		tradableMarkets  domains.TradableMarketsDomain
		expectedSpot     bool
		expectedContract bool
		expectedEmpty    bool
	}{
		{name: "spot only", tradableMarkets: domains.NewTradableMarketsDomain(true, false), expectedSpot: true},
		{name: "contract only", tradableMarkets: domains.NewTradableMarketsDomain(false, true), expectedContract: true},
		{name: "both", tradableMarkets: domains.NewTradableMarketsDomain(true, true), expectedSpot: true, expectedContract: true},
		{name: "neither", tradableMarkets: domains.NewTradableMarketsDomain(false, false), expectedEmpty: true},
		{name: "read back from stored spellings", tradableMarkets: domains.NewTradableMarketsDomainOf([]string{"contract", "spot"}), expectedSpot: true, expectedContract: true},
		{name: "an unknown spelling grants nothing", tradableMarkets: domains.NewTradableMarketsDomainOf([]string{"margin"}), expectedEmpty: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedSpot, testCase.tradableMarkets.Covers(vo.TradableMarketSpot))
			assert.Equal(t, testCase.expectedContract, testCase.tradableMarkets.Covers(vo.TradableMarketContract))
			assert.Equal(t, testCase.expectedEmpty, testCase.tradableMarkets.IsEmpty())
			assert.False(t, testCase.tradableMarkets.Covers(vo.TradableMarketVo("margin")))
		})
	}
}

func TestTradableMarketsDomainNamesTheBotKindsToSwitchOff(t *testing.T) {
	testCases := []struct {
		name            string
		tradableMarkets domains.TradableMarketsDomain
		expectedKinds   []string
	}{
		{name: "both covered switches nothing off", tradableMarkets: domains.NewTradableMarketsDomain(true, true), expectedKinds: []string{}},
		{name: "spot only switches contract bots off", tradableMarkets: domains.NewTradableMarketsDomain(true, false), expectedKinds: []string{"contractKCandle"}},
		// Bots stored before kinds existed have a blank kind and are spot bots.
		{name: "contract only switches spot bots off, old blank ones included", tradableMarkets: domains.NewTradableMarketsDomain(false, true), expectedKinds: []string{"kCandle", ""}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ElementsMatch(t, testCase.expectedKinds, testCase.tradableMarkets.UncoveredBotMarketDataKinds())
		})
	}
}

func TestTradableMarketsDomainRequireCovering(t *testing.T) {
	testCases := []struct {
		name            string
		tradableMarkets domains.TradableMarketsDomain
		botKind         string
		expectedMessage string
	}{
		{name: "spot covers a spot bot", tradableMarkets: domains.NewTradableMarketsDomain(true, false), botKind: "kCandle"},
		{name: "spot covers an old blank-kind bot", tradableMarkets: domains.NewTradableMarketsDomain(true, false), botKind: ""},
		{name: "contract covers a contract bot", tradableMarkets: domains.NewTradableMarketsDomain(false, true), botKind: "contractKCandle"},
		{name: "spot does not cover a contract bot", tradableMarkets: domains.NewTradableMarketsDomain(true, false), botKind: "contractKCandle", expectedMessage: "這組幣安交易金鑰沒有合約交易權限"},
		{name: "contract does not cover a spot bot", tradableMarkets: domains.NewTradableMarketsDomain(false, true), botKind: "kCandle", expectedMessage: "這組幣安交易金鑰沒有現貨交易權限"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.tradableMarkets.RequireCovering(testCase.botKind)

			if testCase.expectedMessage == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, domains.ErrStrategyBotAutoOrderMarketNotCovered)
			assert.ErrorContains(t, err, testCase.expectedMessage)
		})
	}
}
