package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradingStrategyBacktestRequestDto takes sources, conditions, kind and trading mode
// from the trading strategy; the rest is per replay.
type ContractTradingStrategyBacktestRequestDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time

	SignalSources                 []ResolvedSignalSourceDto
	BuyCondition                  TradingStrategyConditionDto
	SellCondition                 TradingStrategyConditionDto
	TradingStrategyMarketDataKind string
	TradingStrategyTradingMode    string

	InitialCapital       decimal.Decimal
	PositionSizingMode   string
	PositionSizingValue  decimal.Decimal
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	EntryCostPercentage  decimal.Decimal
	ExitCostPercentage   decimal.Decimal
	// FillTiming is close (blank) or nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
	Leverage            decimal.Decimal
	SlippagePercentage  decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused, since the symbol's margin ladder
	// decides it.
	MaintenanceMarginRate decimal.Decimal
	// TradingMode is carried only to be refused, since the trading strategy already fixes it.
	TradingMode string
}

func (requestDto ContractTradingStrategyBacktestRequestDto) ToContractBacktestRequestDto(
	sharedAggregationInterval string,
) ContractBacktestRequestDto {
	return ContractBacktestRequestDto{
		Symbol:                requestDto.Symbol,
		AggregationInterval:   sharedAggregationInterval,
		StartTime:             requestDto.StartTime,
		EndTime:               requestDto.EndTime,
		InitialCapital:        requestDto.InitialCapital,
		PositionSizingMode:    requestDto.PositionSizingMode,
		PositionSizingValue:   requestDto.PositionSizingValue,
		StopLossPercentage:    requestDto.StopLossPercentage,
		TakeProfitPercentage:  requestDto.TakeProfitPercentage,
		EntryCostPercentage:   requestDto.EntryCostPercentage,
		ExitCostPercentage:    requestDto.ExitCostPercentage,
		FillTiming:            requestDto.FillTiming,
		ValidationStartTime:   requestDto.ValidationStartTime,
		Leverage:              requestDto.Leverage,
		TradingMode:           requestDto.TradingStrategyTradingMode,
		SlippagePercentage:    requestDto.SlippagePercentage,
		MaintenanceMarginRate: requestDto.MaintenanceMarginRate,
	}
}
