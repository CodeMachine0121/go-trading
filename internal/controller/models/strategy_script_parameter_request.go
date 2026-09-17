package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyScriptParameterRequest is one knob as a caller declares it.
//
// The value is one number and the kind says how to read it, because there are not
// two types here: a person types one number into one box, and what differs is what
// the system does with it.
type StrategyScriptParameterRequest struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// ToWriteDto turns the declaration into the shape the domain settles. Nothing is
// judged here — the blanks around the name and the spelling of the kind are still
// exactly as they arrived.
func (strategyScriptParameterRequest StrategyScriptParameterRequest) ToWriteDto() dto.StrategyScriptParameterWriteDto {
	return dto.StrategyScriptParameterWriteDto{
		Name:         strategyScriptParameterRequest.Name,
		Kind:         strategyScriptParameterRequest.Kind,
		DefaultValue: strategyScriptParameterRequest.DefaultValue,
	}
}

// StrategyScriptParameterValueRequest is what one run says a knob is worth this time.
// It carries no kind: which kind a name was declared as is the strategy script's word.
type StrategyScriptParameterValueRequest struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// ToValueDto turns the supplied value into the shape the domain applies.
func (strategyScriptParameterValueRequest StrategyScriptParameterValueRequest) ToValueDto() dto.StrategyScriptParameterValueDto {
	return dto.StrategyScriptParameterValueDto{
		Name:  strategyScriptParameterValueRequest.Name,
		Value: strategyScriptParameterValueRequest.Value,
	}
}
