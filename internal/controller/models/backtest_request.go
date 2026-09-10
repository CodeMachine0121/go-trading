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
// Like an indicator calculation, it either names a strategy or carries an
// algorithm the caller wrote and has not saved — never both. It declares no kind of
// value either way: a replay reads signals, so there is nothing to choose.
type BacktestRequest struct {
	StrategyID uint   `json:"strategyId"`
	Symbol     string `json:"symbol"`
	// Script and Parameters describe an algorithm the caller wrote and has not
	// saved. They are read only when no strategy is named.
	Script              string                     `json:"script"`
	Parameters          []StrategyParameterRequest `json:"parameters"`
	AggregationInterval string                     `json:"aggregationInterval"`
	StartTime           time.Time                  `json:"startTime"`
	EndTime             time.Time                  `json:"endTime"`
	// ParameterValues are what the strategy's knobs are worth this time, used for
	// this replay only and never written back.
	ParameterValues []StrategyParameterValueRequest `json:"parameterValues"`
	InitialCapital  decimal.Decimal                 `json:"initialCapital"`
	// PositionSizingMode is how much each opening stakes, and PositionSizingValue the
	// figure that goes with it. Staking everything needs no figure, so a caller that
	// chose it may leave the figure out entirely.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
}

// ToParameterWriteDtos hands on the knobs an unsaved algorithm declares.
func (backtestRequest BacktestRequest) ToParameterWriteDtos() []dto.StrategyParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(backtestRequest.Parameters))
	for _, parameterRequest := range backtestRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

// ToRequestDto turns the request into the shape the domain accepts.
func (backtestRequest BacktestRequest) ToRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:              backtestRequest.Symbol,
		AggregationInterval: backtestRequest.AggregationInterval,
		StartTime:           backtestRequest.StartTime,
		EndTime:             backtestRequest.EndTime,
		ParameterValues:     backtestRequest.parameterValueDtos(),
		InitialCapital:      backtestRequest.InitialCapital,
		PositionSizingMode:  backtestRequest.PositionSizingMode,
		PositionSizingValue: backtestRequest.PositionSizingValue,
	}
}

func (backtestRequest BacktestRequest) parameterValueDtos() []dto.StrategyParameterValueDto {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(backtestRequest.ParameterValues))
	for _, valueRequest := range backtestRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
