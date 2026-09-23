package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradingStrategyBacktestRequestDto is one replay of a contract trading
// strategy on a contract account. The sources, the two condition trees, the kind and
// the trading mode are the trading strategy's own; the rest travels with this replay.
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
	// FillTiming is at what price this replay fills its signals: close (blank) or
	// nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
	Leverage            decimal.Decimal
	SlippagePercentage  decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused: on a contract account it is
	// the symbol's ladder that says it, and a figure typed in would otherwise be
	// quietly ignored.
	MaintenanceMarginRate decimal.Decimal
	// TradingMode is what the caller declared for this replay. The trading strategy
	// already says which trading mode it trades by, so anything declared here is
	// refused rather than quietly winning or losing against it.
	TradingMode string
}

// ToContractBacktestRequestDto is this replay as a replay of one contract strategy
// script would be told it, the coarseness and the trading mode being the trading
// strategy's.
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
