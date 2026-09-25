package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest either names a strategy script (whose declared result type and parameters win) or carries an unsaved one, never both; EndTime defaults to now and AggregationInterval to one minute.
type IndicatorCalculationRequest struct {
	StrategyScriptID    uint      `json:"strategyScriptId"`
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	StartTime           time.Time `json:"startTime"`
	EndTime             time.Time `json:"endTime"`
	// Read only when no strategy script is named.
	Script     string                           `json:"script"`
	ResultType string                           `json:"resultType"`
	Parameters []StrategyScriptParameterRequest `json:"parameters"`
	// Used for this run only and never written back to the script.
	ParameterValues []StrategyScriptParameterValueRequest `json:"parameterValues"`
}

func (indicatorCalculationRequest IndicatorCalculationRequest) ToParameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(indicatorCalculationRequest.Parameters))
	for _, parameterRequest := range indicatorCalculationRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

func (indicatorCalculationRequest IndicatorCalculationRequest) ToRequestDto() dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:              indicatorCalculationRequest.Symbol,
		AggregationInterval: indicatorCalculationRequest.AggregationInterval,
		StartTime:           indicatorCalculationRequest.StartTime,
		EndTime:             indicatorCalculationRequest.EndTime,
		ParameterValues:     indicatorCalculationRequest.parameterValueDtos(),
	}
}

func (indicatorCalculationRequest IndicatorCalculationRequest) parameterValueDtos() []dto.StrategyScriptParameterValueDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(indicatorCalculationRequest.ParameterValues))
	for _, valueRequest := range indicatorCalculationRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
