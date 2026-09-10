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
// Like an indicator calculation, it names a strategy rather than carrying an
// algorithm — a replay of a script sent from outside would hide nothing from the
// sender.
type BacktestRequest struct {
	StrategyID          uint      `json:"strategyId"`
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	StartTime           time.Time `json:"startTime"`
	EndTime             time.Time `json:"endTime"`
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
