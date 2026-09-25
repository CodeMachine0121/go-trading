package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ContractBacktestRequestDto struct {
	Symbol               string
	AggregationInterval  string
	StartTime            time.Time
	EndTime              time.Time
	Script               string
	Parameters           []StrategyScriptParameterWriteDto
	ParameterValues      []StrategyScriptParameterValueDto
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
	// Leverage blank or zero is one.
	Leverage    decimal.Decimal
	TradingMode string
	// SlippagePercentage blank is none.
	SlippagePercentage decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused, since the symbol's margin ladder
	// decides it.
	MaintenanceMarginRate decimal.Decimal
}

// ToBacktestRequestDto extracts the spot-compatible part so it is validated by the same
// rules as a spot replay.
func (requestDto ContractBacktestRequestDto) ToBacktestRequestDto() BacktestRequestDto {
	return BacktestRequestDto{
		Symbol:               requestDto.Symbol,
		AggregationInterval:  requestDto.AggregationInterval,
		StartTime:            requestDto.StartTime,
		EndTime:              requestDto.EndTime,
		Script:               requestDto.Script,
		Parameters:           requestDto.Parameters,
		ParameterValues:      requestDto.ParameterValues,
		InitialCapital:       requestDto.InitialCapital,
		PositionSizingMode:   requestDto.PositionSizingMode,
		PositionSizingValue:  requestDto.PositionSizingValue,
		StopLossPercentage:   requestDto.StopLossPercentage,
		TakeProfitPercentage: requestDto.TakeProfitPercentage,
		EntryCostPercentage:  requestDto.EntryCostPercentage,
		ExitCostPercentage:   requestDto.ExitCostPercentage,
		FillTiming:           requestDto.FillTiming,
		ValidationStartTime:  requestDto.ValidationStartTime,
	}
}
