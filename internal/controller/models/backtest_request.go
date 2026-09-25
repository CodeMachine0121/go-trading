package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// BacktestRequest either names a strategy script or carries an unsaved one, never both; it declares no value kind since a replay reads signals.
type BacktestRequest struct {
	StrategyScriptID uint   `json:"strategyScriptId"`
	Symbol           string `json:"symbol"`
	// Read only when no strategy script is named.
	Script              string                           `json:"script"`
	Parameters          []StrategyScriptParameterRequest `json:"parameters"`
	AggregationInterval string                           `json:"aggregationInterval"`
	StartTime           time.Time                        `json:"startTime"`
	EndTime             time.Time                        `json:"endTime"`
	// Used for this replay only and never written back.
	ParameterValues []StrategyScriptParameterValueRequest `json:"parameterValues"`
	InitialCapital  decimal.Decimal                       `json:"initialCapital"`
	// Staking everything needs no PositionSizingValue.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
	// Anything other than spot is refused rather than silently treated as spot.
	TradingMode string `json:"tradingMode"`
	// Both empty means no simulated exits.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Only carried to refuse borrowing: empty, zero and one mean fully paid, anything above one is refused.
	Leverage decimal.Decimal `json:"leverage"`
	// Carried only to be refused, since nothing here borrows.
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
	// Percentages of traded notional; both empty means free trading, and an empty exit cost equals the entry cost.
	EntryCostPercentage decimal.Decimal `json:"entryCostPercentage"`
	ExitCostPercentage  decimal.Decimal `json:"exitCostPercentage"`
	// FillTiming is close (default, the signalling bar's close) or nextOpen (the next bar's open).
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime, when given, splits the replay into in-sample and validation parts, each starting from the initial capital.
	ValidationStartTime time.Time `json:"validationStartTime"`
}

func (backtestRequest BacktestRequest) ToParameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(backtestRequest.Parameters))
	for _, parameterRequest := range backtestRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

func (backtestRequest BacktestRequest) ToRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:               backtestRequest.Symbol,
		AggregationInterval:  backtestRequest.AggregationInterval,
		StartTime:            backtestRequest.StartTime,
		EndTime:              backtestRequest.EndTime,
		ParameterValues:      backtestRequest.parameterValueDtos(),
		InitialCapital:       backtestRequest.InitialCapital,
		PositionSizingMode:   backtestRequest.PositionSizingMode,
		PositionSizingValue:  backtestRequest.PositionSizingValue,
		TradingMode:          backtestRequest.TradingMode,
		StopLossPercentage:   backtestRequest.StopLossPercentage,
		TakeProfitPercentage: backtestRequest.TakeProfitPercentage,

		Leverage:              backtestRequest.Leverage,
		MaintenanceMarginRate: backtestRequest.MaintenanceMarginRate,

		EntryCostPercentage: backtestRequest.EntryCostPercentage,
		ExitCostPercentage:  backtestRequest.ExitCostPercentage,
		FillTiming:          backtestRequest.FillTiming,
		ValidationStartTime: backtestRequest.ValidationStartTime,
	}
}

func (backtestRequest BacktestRequest) parameterValueDtos() []dto.StrategyScriptParameterValueDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(backtestRequest.ParameterValues))
	for _, valueRequest := range backtestRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
