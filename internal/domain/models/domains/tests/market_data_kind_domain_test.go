package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketDataKindDomainRefusesToRunWhereTheOtherKindOfMarketIsHandedOver(t *testing.T) {
	testCases := []struct {
		name          string
		declared      string
		expected      vo.MarketDataKindVo
		expectedWords string
	}{
		{name: "a spot script named for a contract calculation", declared: "kCandle",
			expected: vo.MarketDataKindContractKCandle, expectedWords: "這支策略腳本吃的是 K 線"},
		{name: "a contract script named for a spot calculation", declared: "contractKCandle",
			expected: vo.MarketDataKindKCandle, expectedWords: "這支策略腳本吃的是合約行情"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			marketDataKind, declarationError := domains.NewMarketDataKindDomain(testCase.declared)
			require.NoError(t, declarationError)

			mismatchError := marketDataKind.RequireRunnableAs(testCase.expected)

			require.ErrorIs(t, mismatchError, domains.ErrStrategyScriptMarketDataKindMismatch)
			assert.Contains(t, mismatchError.Error(), testCase.expectedWords)
		})
	}
}

func TestMarketDataKindDomainRunsWhereItsOwnKindOfMarketIsHandedOver(t *testing.T) {
	marketDataKind, declarationError := domains.NewMarketDataKindDomain("")
	require.NoError(t, declarationError)

	assert.NoError(t, marketDataKind.RequireRunnableAs(vo.MarketDataKindKCandle))
}
