package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// BacktestRequest is the body a caller sends to replay a strategy over a stretch of
// market that has already happened.
//
// It carries no indicator value kind. A replay reads one number per candle — the
// signal — so there is nothing here for a caller to declare and nothing to get wrong.
type BacktestRequest struct {
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	StartTime           time.Time `json:"startTime"`
	EndTime             time.Time `json:"endTime"`
	Script              string    `json:"script"`
	// Parameters are the algorithm's knobs as declared, and ParameterValues what they
	// are worth this time. Both arrive with the run rather than being looked up,
	// because what is replayed is a script — it may never have been saved.
	Parameters      []StrategyParameterRequest      `json:"parameters"`
	ParameterValues []StrategyParameterValueRequest `json:"parameterValues"`
	InitialCapital  decimal.Decimal                 `json:"initialCapital"`
	// PositionSizingMode is how much each opening stakes, and PositionSizingValue the
	// figure that goes with it. Staking everything needs no figure, so a caller that
	// chose it may leave the figure out entirely.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
}

// ToRequestDto turns the request into the shape the domain accepts.
func (backtestRequest BacktestRequest) ToRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:              backtestRequest.Symbol,
		AggregationInterval: backtestRequest.AggregationInterval,
		StartTime:           backtestRequest.StartTime,
		EndTime:             backtestRequest.EndTime,
		Script:              backtestRequest.Script,
		Parameters:          backtestRequest.parameterWriteDtos(),
		ParameterValues:     backtestRequest.parameterValueDtos(),
		InitialCapital:      backtestRequest.InitialCapital,
		PositionSizingMode:  backtestRequest.PositionSizingMode,
		PositionSizingValue: backtestRequest.PositionSizingValue,
	}
}

func (backtestRequest BacktestRequest) parameterWriteDtos() []dto.StrategyParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(backtestRequest.Parameters))
	for _, parameterRequest := range backtestRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

func (backtestRequest BacktestRequest) parameterValueDtos() []dto.StrategyParameterValueDto {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(backtestRequest.ParameterValues))
	for _, valueRequest := range backtestRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
