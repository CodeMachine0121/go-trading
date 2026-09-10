package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
//
// It names a strategy rather than carrying an algorithm. That is what lets one
// person run another's published strategy without reading it: a script sent from
// outside is a script the sender already has, and then keeping it hidden is only a
// matter of a screen not showing it.
//
// The kind of value is not here either — the strategy declares it, and declaring it
// again per run would be a second answer that can disagree with the first.
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
	// ParameterValues are what the strategy's knobs are worth this time. They are
	// the caller's own and are used for this run only — running somebody else's
	// strategy never writes anything back to it.
	ParameterValues []StrategyParameterValueRequest `json:"parameterValues"`
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
