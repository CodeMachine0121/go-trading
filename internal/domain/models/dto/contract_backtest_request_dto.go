package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractBacktestRequestDto is one replay of a contract strategy script on a contract
// account: everything a spot replay is told, plus the leverage, the trading mode and
// the slippage.
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
	// FillTiming is at what price this replay fills its signals: close (blank) or
	// nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
	// Leverage blank or zero is one.
	Leverage    decimal.Decimal
	TradingMode string
	// SlippagePercentage blank is none.
	SlippagePercentage decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused: on a contract account it is
	// the symbol's ladder that says it, and a figure typed in would otherwise be
	// quietly ignored.
	MaintenanceMarginRate decimal.Decimal
}

// ToBacktestRequestDto is the part of this request a spot replay understands too —
// the stretch, the capital, the sizing, the exits and the costs — so that those are
// read by the very rules a spot replay reads them by.
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
