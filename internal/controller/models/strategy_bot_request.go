package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBotRequest serves create and full rewrite; the ID and owner never come from the body, and rules are referenced by trading strategy rather than embedded.
type StrategyBotRequest struct {
	Name string `json:"name"`
	// Symbols is the complete list of symbols the bot watches, 1 to 10; on rewrite, symbols left out are removed and new ones added.
	Symbols []string `json:"symbols"`
	// MarketDataKind is kCandle (default on create) or contractKCandle; omitted on rewrite keeps the existing kind, and naming the other kind is refused.
	MarketDataKind         string `json:"marketDataKind"`
	TradingStrategyID      uint   `json:"tradingStrategyId"`
	TriggerIntervalMinutes int    `json:"triggerIntervalMinutes"`
	// May be omitted entirely, in which case the bot sends its plain message.
	PositionPlan PositionPlanRequest `json:"positionPlan"`
	// AutoOrderEntrySlippageTolerancePercentage is in percent, 0 to 5; omitted means 0.1 on create and unchanged on rewrite.
	AutoOrderEntrySlippageTolerancePercentage decimal.NullDecimal `json:"autoOrderEntrySlippageTolerancePercentage"`
}

type PositionPlanRequest struct {
	// Capital is the bot's total, shared among its watched symbols; on a contract bot it is margin, and the notional is margin × leverage.
	Capital     decimal.Decimal `json:"capital"`
	SizingMode  string          `json:"sizingMode"`
	SizingValue decimal.Decimal `json:"sizingValue"`
	// Leverage applies to contract bots; on a spot bot it is read only so borrowing can be refused.
	Leverage             decimal.Decimal `json:"leverage"`
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// CapitalAllocations is optional: given, it names every watched symbol once; left out or empty, the capital is split equally (or, on rewrite with unchanged symbols, kept as stored).
	CapitalAllocations []CapitalAllocationRequest `json:"capitalAllocations"`
}

type CapitalAllocationRequest struct {
	Symbol string `json:"symbol"`
	// AllocationPercentage is this symbol's share of the capital in percent: above 0, at most 100, two decimal places at most.
	AllocationPercentage decimal.Decimal `json:"allocationPercentage"`
}

func (capitalAllocationRequest CapitalAllocationRequest) ToDto() dto.StrategyBotCapitalAllocationDto {
	return dto.StrategyBotCapitalAllocationDto{
		Symbol:               capitalAllocationRequest.Symbol,
		AllocationPercentage: capitalAllocationRequest.AllocationPercentage,
	}
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
	capitalAllocations := make([]dto.StrategyBotCapitalAllocationDto, 0, len(strategyBotRequest.PositionPlan.CapitalAllocations))
	for _, capitalAllocationRequest := range strategyBotRequest.PositionPlan.CapitalAllocations {
		capitalAllocations = append(capitalAllocations, capitalAllocationRequest.ToDto())
	}

	return dto.StrategyBotWriteDto{
		ID:                     id,
		Name:                   strategyBotRequest.Name,
		Symbols:                strategyBotRequest.Symbols,
		MarketDataKind:         strategyBotRequest.MarketDataKind,
		TradingStrategyID:      strategyBotRequest.TradingStrategyID,
		TriggerIntervalMinutes: strategyBotRequest.TriggerIntervalMinutes,
		PositionPlan:           strategyBotRequest.PositionPlan.ToSettingsDto(),
		CapitalAllocations:     capitalAllocations,
		DeclaredLeverage:       strategyBotRequest.PositionPlan.Leverage,
		AutoOrderEntrySlippageTolerancePercentage: strategyBotRequest.AutoOrderEntrySlippageTolerancePercentage,
	}
}
