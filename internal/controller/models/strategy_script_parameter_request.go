package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type StrategyScriptParameterRequest struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// ToWriteDto passes the declaration on untouched; validation is the domain's.
func (strategyScriptParameterRequest StrategyScriptParameterRequest) ToWriteDto() dto.StrategyScriptParameterWriteDto {
	return dto.StrategyScriptParameterWriteDto{
		Name:         strategyScriptParameterRequest.Name,
		Kind:         strategyScriptParameterRequest.Kind,
		DefaultValue: strategyScriptParameterRequest.DefaultValue,
	}
}

// StrategyScriptParameterValueRequest carries no kind, since the strategy script's declaration decides it.
type StrategyScriptParameterValueRequest struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

func (strategyScriptParameterValueRequest StrategyScriptParameterValueRequest) ToValueDto() dto.StrategyScriptParameterValueDto {
	return dto.StrategyScriptParameterValueDto{
		Name:  strategyScriptParameterValueRequest.Name,
		Value: strategyScriptParameterValueRequest.Value,
	}
}
