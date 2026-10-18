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
	TradingMode           string
	TimeAllowance         time.Duration
	DefaultWarmupBarCount int
}

func (requestDto ContractTradingStrategyBacktestRequestDto) ToContractBacktestRequestDto(
	paceAggregationInterval string,
) ContractBacktestRequestDto {
	return ContractBacktestRequestDto{
		Symbol:                requestDto.Symbol,
		AggregationInterval:   paceAggregationInterval,
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
		TimeAllowance:         requestDto.TimeAllowance,
		DefaultWarmupBarCount: requestDto.DefaultWarmupBarCount,
	}
}

// WarmingUpByDefault fills in the server's default warm-up when the request names none, so judging a replay and running it read the same stretch.
func (requestDto ContractTradingStrategyBacktestRequestDto) WarmingUpByDefault(defaultWarmupBarCount int) ContractTradingStrategyBacktestRequestDto {
	if requestDto.DefaultWarmupBarCount == 0 {
		requestDto.DefaultWarmupBarCount = defaultWarmupBarCount
	}

	return requestDto
}
