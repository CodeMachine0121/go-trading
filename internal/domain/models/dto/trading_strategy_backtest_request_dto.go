package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// TradingStrategyBacktestRequestDto has no interval because each signal source carries its own.
type TradingStrategyBacktestRequestDto struct {
	Symbol string
	// StartTime and EndTime are inclusive; a future end is read as now.
	StartTime time.Time
	EndTime   time.Time
	// SignalSources arrive with scripts already resolved and access-checked by the application.
	SignalSources []ResolvedSignalSourceDto
	// TradingStrategyMarketDataKind lets contract trading strategies be refused here.
	TradingStrategyMarketDataKind string
	BuyCondition                  TradingStrategyConditionDto
	SellCondition                 TradingStrategyConditionDto
	// InitialCapital must be above zero.
	InitialCapital      decimal.Decimal
	PositionSizingMode  string
	PositionSizingValue decimal.Decimal
	// StopLossPercentage and TakeProfitPercentage are optional; zero means no such exit.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	// TradingMode is carried only so a request for an unsupported mode can be refused.
	TradingMode string
	// Leverage of nothing, zero or one means fully paid; anything above one is carried only
	// to be refused.
	Leverage decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused, since an unleveraged account
	// cannot be liquidated.
	MaintenanceMarginRate decimal.Decimal
	// EntryCostPercentage and ExitCostPercentage are optional; a missing exit cost defaults
	// to the entry cost.
	EntryCostPercentage decimal.Decimal
	ExitCostPercentage  decimal.Decimal
	// FillTiming is close (blank) or nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
}

// ResolvedSignalSourceDto has its script pre-fetched because read access is checked outside
// the replay.
type ResolvedSignalSourceDto struct {
	Label               string
	AggregationInterval string
	Script              string
	MarketDataKind      string
	Parameters          []StrategyScriptParameterWriteDto
	ParameterValues     []StrategyScriptParameterValueDto
}

// ToBacktestRequestDto builds the shared replay conditions once the signal sources' common
// interval is known; scripts and parameters stay per source.
func (requestDto TradingStrategyBacktestRequestDto) ToBacktestRequestDto(
	sharedAggregationInterval string,
) BacktestRequestDto {
	return BacktestRequestDto{
		Symbol:               requestDto.Symbol,
		AggregationInterval:  sharedAggregationInterval,
		StartTime:            requestDto.StartTime,
		EndTime:              requestDto.EndTime,
		InitialCapital:       requestDto.InitialCapital,
		PositionSizingMode:   requestDto.PositionSizingMode,
		PositionSizingValue:  requestDto.PositionSizingValue,
		TradingMode:          requestDto.TradingMode,
		StopLossPercentage:   requestDto.StopLossPercentage,
		TakeProfitPercentage: requestDto.TakeProfitPercentage,

		Leverage:              requestDto.Leverage,
		MaintenanceMarginRate: requestDto.MaintenanceMarginRate,

		EntryCostPercentage: requestDto.EntryCostPercentage,
		ExitCostPercentage:  requestDto.ExitCostPercentage,
		FillTiming:          requestDto.FillTiming,
		ValidationStartTime: requestDto.ValidationStartTime,
	}
}
