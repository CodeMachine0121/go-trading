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

	// Both times are handed out in universal time whatever zone they were read back
	// in, so two people in two places read the same moment.
	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), botDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC), botDto.UpdatedAt)

	// A list of bots is read to answer one question — which of these needs looking
	// at — so every part of that answer travels with the bot.
	assert.Equal(t, uint(7), botDto.OwnerID)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
	assert.Equal(t, string(vo.SignalBuy), botDto.LastSentSignal)
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), botDto.HaltReason)
	assert.True(t, botDto.Conflicting)
}

// A bot names its rules and carries their current name beside the identifier, so a
// list of bots says what each one is doing without a second read per bot.
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

// The name is read through the association every time rather than copied onto the
// bot, so a bot read without it says nothing rather than something out of date.
func TestStrategyBotToDtoKeepsNoCopyOfTheRulesName(t *testing.T) {
	botDto := StrategyBot{ID: 3, TradingStrategyID: 9}.ToDto()

	assert.Equal(t, uint(9), botDto.TradingStrategyID)
	assert.Empty(t, botDto.TradingStrategyName)
}
