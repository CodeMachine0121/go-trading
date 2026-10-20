package entities_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestStrategyBotToDtoCarriesWhatTheListIsReadFor(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破",
		TradingStrategyID:      9,
		TradingStrategy:        TradingStrategy{ID: 9, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		HaltReason:             string(vo.StrategyBotHaltScriptFailed),
		CreatedAt:              time.Date(2026, 9, 16, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		UpdatedAt:              time.Date(2026, 9, 16, 9, 0, 0, 0, time.FixedZone("CST", 8*3600)),
	}

	botDto := strategyBot.ToDto()

	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), botDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC), botDto.UpdatedAt)

	assert.Equal(t, uint(7), botDto.OwnerID)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), botDto.HaltReason)
}

func TestStrategyBotToDtoNamesTheRulesItFollows(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破",
		TradingStrategyID: 9,
		TradingStrategy:   TradingStrategy{ID: 9, Name: "黃金交叉"},
	}

	botDto := strategyBot.ToDto()

	assert.Equal(t, uint(9), botDto.TradingStrategyID)
	assert.Equal(t, "黃金交叉", botDto.TradingStrategyName)
}

// A bot read without its association shows no name rather than a stale copy.
func TestStrategyBotToDtoKeepsNoCopyOfTheRulesName(t *testing.T) {
	botDto := StrategyBot{ID: 3, TradingStrategyID: 9}.ToDto()

	assert.Equal(t, uint(9), botDto.TradingStrategyID)
	assert.Empty(t, botDto.TradingStrategyName)
}

func TestStrategyBotToDtoCarriesTheAutoOrderSwitch(t *testing.T) {
	testCases := []struct {
		name             string
		autoOrderEnabled bool
	}{
		{name: "switched on", autoOrderEnabled: true},
		{name: "switched off, as every bot starts", autoOrderEnabled: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategyBotDto := StrategyBot{AutoOrderEnabled: testCase.autoOrderEnabled}.ToDto()

			assert.Equal(t, testCase.autoOrderEnabled, strategyBotDto.AutoOrderEnabled)
		})
	}
}

func TestStrategyBotToDtoShowsEachWatchedSymbolInTheBotsOrder(t *testing.T) {
	testCases := []struct {
		name                      string
		marketDataKind            string
		expectedAutoOrderPosition *dto.AutoOrderPositionDto
	}{
		{
			name:                      "a contract bot shows what auto orders opened on the symbol",
			marketDataKind:            string(vo.MarketDataKindContractKCandle),
			expectedAutoOrderPosition: &dto.AutoOrderPositionDto{Direction: "long", Quantity: decimal.RequireFromString("0.01")},
		},
		{
			name:                      "a spot bot opens nothing, so shows no position",
			marketDataKind:            string(vo.MarketDataKindKCandle),
			expectedAutoOrderPosition: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategyBot := StrategyBot{
				MarketDataKind: testCase.marketDataKind,
				WatchedSymbols: []StrategyBotWatchedSymbol{
					{
						Symbol: "BTCUSDT", LastSentSignal: string(vo.SignalBuy),
						AutoOrderPositionDirection: "long", AutoOrderPositionQuantity: decimal.RequireFromString("0.01"),
					},
					{Symbol: "ETHUSDT", HaltReason: string(vo.StrategyBotHaltScriptFailed), Conflicting: true},
				},
			}

			botDto := strategyBot.ToDto()

			assert.Equal(t, []dto.StrategyBotWatchedSymbolDto{
				{Symbol: "BTCUSDT", LastSentSignal: string(vo.SignalBuy), AutoOrderPosition: testCase.expectedAutoOrderPosition},
				{
					Symbol: "ETHUSDT", HaltReason: string(vo.StrategyBotHaltScriptFailed), Conflicting: true,
					AutoOrderPosition: emptyPositionFor(testCase.expectedAutoOrderPosition),
				},
			}, botDto.WatchedSymbols)
		})
	}
}

// emptyPositionFor is a flat position on a contract bot and none on a spot bot.
func emptyPositionFor(contractPosition *dto.AutoOrderPositionDto) *dto.AutoOrderPositionDto {
	if contractPosition == nil {
		return nil
	}

	return &dto.AutoOrderPositionDto{Direction: "", Quantity: decimal.Decimal{}}
}
