package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

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
	// PositionPlan is what this bot is to suggest putting down each round. The whole
	// group may be left out: a bot without one sends the message it sent before
	// position plans existed.
	PositionPlan PositionPlanRequest `json:"positionPlan"`
}

// PositionPlanRequest is the five figures behind a suggested position.
//
// The money arrives as an exact decimal rather than a JSON number, for the reason it
// does everywhere else here: the leverage and the two distances all multiply into a
// price somebody places an order at.
type PositionPlanRequest struct {
	Capital              decimal.Decimal `json:"capital"`
	SizingMode           string          `json:"sizingMode"`
	SizingValue          decimal.Decimal `json:"sizingValue"`
	Leverage             decimal.Decimal `json:"leverage"`
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
}

// ToSettingsDto hands the five figures on untouched. Reading them — a blank sizing
// mode meaning stake everything, a leverage of nothing meaning one — is the domain's
// job, the same way it is for every other declared spelling here.
func (positionPlanRequest PositionPlanRequest) ToSettingsDto() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              positionPlanRequest.Capital,
		SizingMode:           positionPlanRequest.SizingMode,
		SizingValue:          positionPlanRequest.SizingValue,
		Leverage:             positionPlanRequest.Leverage,
		StopLossPercentage:   positionPlanRequest.StopLossPercentage,
		TakeProfitPercentage: positionPlanRequest.TakeProfitPercentage,
	}
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
		PositionPlan:           strategyBotRequest.PositionPlan.ToSettingsDto(),
	}
}
