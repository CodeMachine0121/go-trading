package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBotRequest serves create and full rewrite; the ID and owner never come from the body, and rules are referenced by trading strategy rather than embedded.
type StrategyBotRequest struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	// MarketDataKind is kCandle (default on create) or contractKCandle; omitted on rewrite keeps the existing kind, and naming the other kind is refused.
	MarketDataKind         string `json:"marketDataKind"`
	TradingStrategyID      uint   `json:"tradingStrategyId"`
	TriggerIntervalMinutes int    `json:"triggerIntervalMinutes"`
	// May be omitted entirely, in which case the bot sends its plain message.
	PositionPlan PositionPlanRequest `json:"positionPlan"`
}

type PositionPlanRequest struct {
	Capital     decimal.Decimal `json:"capital"`
	SizingMode  string          `json:"sizingMode"`
	SizingValue decimal.Decimal `json:"sizingValue"`
	// Leverage applies to contract bots; on a spot bot it is read only so borrowing can be refused.
	Leverage             decimal.Decimal `json:"leverage"`
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
}

// ToSettingsDto passes the stored figures on untouched for the domain to interpret; declared leverage is carried separately because its handling depends on bot kind.
func (positionPlanRequest PositionPlanRequest) ToSettingsDto() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              positionPlanRequest.Capital,
		SizingMode:           positionPlanRequest.SizingMode,
		SizingValue:          positionPlanRequest.SizingValue,
		StopLossPercentage:   positionPlanRequest.StopLossPercentage,
		TakeProfitPercentage: positionPlanRequest.TakeProfitPercentage,
	}
}

// ToWriteDto takes the bot ID from the argument (zero means new); the owner is set by the application.
func (strategyBotRequest StrategyBotRequest) ToWriteDto(id uint) dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		ID:                     id,
		Name:                   strategyBotRequest.Name,
		Symbol:                 strategyBotRequest.Symbol,
		MarketDataKind:         strategyBotRequest.MarketDataKind,
		TradingStrategyID:      strategyBotRequest.TradingStrategyID,
		TriggerIntervalMinutes: strategyBotRequest.TriggerIntervalMinutes,
		PositionPlan:           strategyBotRequest.PositionPlan.ToSettingsDto(),
		DeclaredLeverage:       strategyBotRequest.PositionPlan.Leverage,
	}
}
