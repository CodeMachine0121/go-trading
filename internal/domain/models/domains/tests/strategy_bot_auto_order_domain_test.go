package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyBotAutoOrderDomainRequireEnableable(t *testing.T) {
	testCases := []struct {
		name            string
		marketDataKind  string
		keyStatus       dto.BinanceTradingKeyStatusDto
		expectedError   error
		expectedMessage string
	}{
		{
			name: "a spot bot with a spot key", marketDataKind: "kCandle",
			keyStatus: dto.BinanceTradingKeyStatusDto{Configured: true, TradableMarkets: []string{"spot"}},
		},
		{
			name: "an old blank-kind bot is a spot bot", marketDataKind: "",
			keyStatus: dto.BinanceTradingKeyStatusDto{Configured: true, TradableMarkets: []string{"spot"}},
		},
		{
			name: "a contract bot with a contract key", marketDataKind: "contractKCandle",
			keyStatus: dto.BinanceTradingKeyStatusDto{Configured: true, TradableMarkets: []string{"contract"}},
		},
		{
			name: "no key at all", marketDataKind: "kCandle",
			keyStatus:       dto.BinanceTradingKeyStatusDto{Configured: false, TradableMarkets: []string{}},
			expectedError:   domains.ErrStrategyBotAutoOrderKeyNotConfigured,
			expectedMessage: "請先完成幣安交易金鑰設定",
		},
		{
			name: "a contract bot with a spot-only key", marketDataKind: "contractKCandle",
			keyStatus:       dto.BinanceTradingKeyStatusDto{Configured: true, TradableMarkets: []string{"spot"}},
			expectedError:   domains.ErrStrategyBotAutoOrderMarketNotCovered,
			expectedMessage: "這組幣安交易金鑰沒有合約交易權限",
		},
		{
			name: "a spot bot with a contract-only key", marketDataKind: "kCandle",
			keyStatus:       dto.BinanceTradingKeyStatusDto{Configured: true, TradableMarkets: []string{"contract"}},
			expectedError:   domains.ErrStrategyBotAutoOrderMarketNotCovered,
			expectedMessage: "這組幣安交易金鑰沒有現貨交易權限",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := domains.NewStrategyBotAutoOrderDomain(
				entities.StrategyBot{MarketDataKind: testCase.marketDataKind},
			).RequireEnableable(testCase.keyStatus)

			if testCase.expectedError == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, testCase.expectedError)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyBotAutoOrderDomainIsEnabled(t *testing.T) {
	assert.True(t, domains.NewStrategyBotAutoOrderDomain(entities.StrategyBot{AutoOrderEnabled: true}).IsEnabled())
	assert.False(t, domains.NewStrategyBotAutoOrderDomain(entities.StrategyBot{}).IsEnabled())
}
