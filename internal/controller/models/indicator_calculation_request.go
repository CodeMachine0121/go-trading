package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
//
// StartTime says where the stretch of market to read begins and is the one field
// with nothing sensible to fall back on. AggregationInterval, ResultType and EndTime
// may each be left out: the calculation then reads one-minute candles, produces one
// number per indicator, and computes up to now.
type IndicatorCalculationRequest struct {
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	StartTime           time.Time `json:"startTime"`
	EndTime             time.Time `json:"endTime"`
	Script              string    `json:"script"`
	ResultType          string    `json:"resultType"`
	// Parameters are the algorithm's knobs as declared, and ParameterValues what
	// they are worth this time. Both arrive with the run rather than being looked
	// up, because what runs here is a script — it may never have been saved.
	Parameters      []StrategyParameterRequest      `json:"parameters"`
	ParameterValues []StrategyParameterValueRequest `json:"parameterValues"`
}

// ToRequestDto turns the request into the shape the domain accepts.
func (indicatorCalculationRequest IndicatorCalculationRequest) ToRequestDto() dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:              indicatorCalculationRequest.Symbol,
		AggregationInterval: indicatorCalculationRequest.AggregationInterval,
		StartTime:           indicatorCalculationRequest.StartTime,
		EndTime:             indicatorCalculationRequest.EndTime,
		Script:              indicatorCalculationRequest.Script,
		ResultType:          indicatorCalculationRequest.ResultType,
		Parameters:          indicatorCalculationRequest.parameterWriteDtos(),
		ParameterValues:     indicatorCalculationRequest.parameterValueDtos(),
	}
}

func (indicatorCalculationRequest IndicatorCalculationRequest) parameterWriteDtos() []dto.StrategyParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(indicatorCalculationRequest.Parameters))
	for _, parameterRequest := range indicatorCalculationRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

func (indicatorCalculationRequest IndicatorCalculationRequest) parameterValueDtos() []dto.StrategyParameterValueDto {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(indicatorCalculationRequest.ParameterValues))
	for _, valueRequest := range indicatorCalculationRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
