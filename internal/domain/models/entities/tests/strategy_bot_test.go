package entities_test

import (
	"testing"
	"time"

	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestStrategyBotToDtoCarriesWhatTheListIsReadFor(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破", Symbol: "BTCUSDT",
		TradingStrategyID:      9,
		TradingStrategy:        TradingStrategy{ID: 9, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		LastSentSignal:         string(vo.SignalBuy),
		HaltReason:             string(vo.StrategyBotHaltScriptFailed),
		Conflicting:            true,
		CreatedAt:              time.Date(2026, 9, 16, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		UpdatedAt:              time.Date(2026, 9, 16, 9, 0, 0, 0, time.FixedZone("CST", 8*3600)),
	}

	botDto := strategyBot.ToDto()

	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), botDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC), botDto.UpdatedAt)

	assert.Equal(t, uint(7), botDto.OwnerID)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
	assert.Equal(t, string(vo.SignalBuy), botDto.LastSentSignal)
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), botDto.HaltReason)
	assert.True(t, botDto.Conflicting)
}

func TestStrategyBotToDtoNamesTheRulesItFollows(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破", Symbol: "BTCUSDT",
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
