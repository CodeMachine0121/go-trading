package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
//
// AggregationInterval, ResultType and EndTime may each be left out: the calculation
// then reads five-minute candles, produces one number per indicator, and computes up
// to now — which is exactly what it did before any of the three could be declared.
type IndicatorCalculationRequest struct {
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	CandleCount         int       `json:"candleCount"`
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
		CandleCount:         indicatorCalculationRequest.CandleCount,
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
