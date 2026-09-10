package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
//
// It either names a strategy or carries an algorithm, never both. Naming one is
// what lets a person run another's published strategy without reading it. Carrying
// one is for an algorithm the caller just wrote and has not saved — their own text,
// hidden from nobody. The rule this protects is that nobody receives an algorithm
// they may not read, not that nobody may supply one.
//
// The kind of value and the knobs belong with the algorithm: a named strategy has
// already declared them, so sending them again would be a second answer that can
// disagree with the first, and they are ignored. An unsaved algorithm has nobody
// to have declared them, so they arrive here.
//
// StartTime says where the stretch of market to read begins and is the one field
// with nothing sensible to fall back on. AggregationInterval and EndTime may each be
// left out: the calculation then reads one-minute candles and computes up to now.
type IndicatorCalculationRequest struct {
	StrategyID          uint      `json:"strategyId"`
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	StartTime           time.Time `json:"startTime"`
	EndTime             time.Time `json:"endTime"`
	// Script, ResultType and Parameters describe an algorithm the caller wrote and
	// has not saved. They are read only when no strategy is named.
	Script     string                     `json:"script"`
	ResultType string                     `json:"resultType"`
	Parameters []StrategyParameterRequest `json:"parameters"`
	// ParameterValues are what the knobs are worth this time. They are the caller's
	// own and are used for this run only — running somebody else's strategy never
	// writes anything back to it.
	ParameterValues []StrategyParameterValueRequest `json:"parameterValues"`
}

// ToParameterWriteDtos hands on the knobs an unsaved algorithm declares. A named
// strategy has its own, and these are then never read.
func (indicatorCalculationRequest IndicatorCalculationRequest) ToParameterWriteDtos() []dto.StrategyParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(indicatorCalculationRequest.Parameters))
	for _, parameterRequest := range indicatorCalculationRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

// ToRequestDto turns the request into the shape the domain accepts.
func (indicatorCalculationRequest IndicatorCalculationRequest) ToRequestDto() dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:              indicatorCalculationRequest.Symbol,
		AggregationInterval: indicatorCalculationRequest.AggregationInterval,
		StartTime:           indicatorCalculationRequest.StartTime,
		EndTime:             indicatorCalculationRequest.EndTime,
		ParameterValues:     indicatorCalculationRequest.parameterValueDtos(),
	}
}

func (indicatorCalculationRequest IndicatorCalculationRequest) parameterValueDtos() []dto.StrategyParameterValueDto {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(indicatorCalculationRequest.ParameterValues))
	for _, valueRequest := range indicatorCalculationRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
