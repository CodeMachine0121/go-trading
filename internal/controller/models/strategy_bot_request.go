package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyBotRequest is the body a caller sends to build or rewrite a strategy bot.
//
// One shape serves both, because a rewrite replaces everything a bot is. Which bot is
// meant comes from the path, never from the body, and neither does who owns it — a
// body that could name its own owner is a body that could claim somebody else's.
//
// The rules are named, not given. A bot that carried its own copy of them would be a
// bot nobody could share, and a body with nowhere to put a condition is how that is
// made true of the shape rather than of the code reading it.
//
// Nothing about a bot's life is here either: run state, when it is next due, what it
// last sent and why it halted are things that happen to a bot, not things a caller
// sets.
type StrategyBotRequest struct {
	Name                   string `json:"name"`
	Symbol                 string `json:"symbol"`
	TradingStrategyID      uint   `json:"tradingStrategyId"`
	TriggerIntervalMinutes int    `json:"triggerIntervalMinutes"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking which bot
// is meant from the argument. A zero identifier means a bot that does not exist yet.
//
// The owner is not taken here at all: it is settled by the application from whoever
// is signed in, and on a rewrite from what is already stored.
func (strategyBotRequest StrategyBotRequest) ToWriteDto(id uint) dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		ID:                     id,
		Name:                   strategyBotRequest.Name,
		Symbol:                 strategyBotRequest.Symbol,
		TradingStrategyID:      strategyBotRequest.TradingStrategyID,
		TriggerIntervalMinutes: strategyBotRequest.TriggerIntervalMinutes,
	}
}
