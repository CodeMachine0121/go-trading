package application

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// strategyParameterAssistantArgument is one knob as the assistant declares it.
type strategyParameterAssistantArgument struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// strategyWriteAssistantArguments is what the assistant sends to save or rewrite a
// strategy.
//
// One shape serves both, exactly as it does for a person: a rewrite replaces
// everything a strategy remembers, so there is no field the assistant may set on one
// path and not the other. Which strategy is meant is the identifier, and a zero one
// means none yet.
type strategyWriteAssistantArguments struct {
	StrategyID uint                                 `json:"strategyId"`
	Name       string                               `json:"name"`
	Script     string                               `json:"script"`
	ResultType string                               `json:"resultType"`
	Parameters []strategyParameterAssistantArgument `json:"parameters"`
}

// ToWriteDto turns what the assistant declared into the shape the domain judges,
// taking the identity from the argument so that the capability calling this decides
// whether a strategy is being saved or rewritten.
func (strategyWriteAssistantArguments strategyWriteAssistantArguments) ToWriteDto(id uint) dto.StrategyWriteDto {
	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(strategyWriteAssistantArguments.Parameters))
	for _, parameter := range strategyWriteAssistantArguments.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, dto.StrategyParameterWriteDto{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			DefaultValue: parameter.DefaultValue,
		})
	}

	return dto.StrategyWriteDto{
		ID:         id,
		Name:       strategyWriteAssistantArguments.Name,
		Script:     strategyWriteAssistantArguments.Script,
		ResultType: strategyWriteAssistantArguments.ResultType,
		Parameters: parameterWriteDtos,
	}
}

// strategyWriteArgumentSchema is the arguments both writing capabilities take. It is
// written once because they take the same ones — the only difference is whether the
// identifier is required, and each says that for itself.
const strategyWriteArgumentSchema = `` +
	`"name":{"type":"string","description":"策略名稱，不得空白、不得與既有策略重複，上限 128 字"},` +
	`"script":{"type":"string","description":"指標算式（Go 函式本文），不得空白"},` +
	`"resultType":{"type":"string","enum":["float","floatList","bool","boolList"],"description":"指標值種類，未給視為 float"},` +
	`"parameters":{"type":"array","description":"這支策略自己的參數","items":{"type":"object","properties":{` +
	`"name":{"type":"string"},` +
	`"kind":{"type":"string","enum":["lookbackCount","number","boolean"],"description":"lookbackCount 是要看過去幾根，number 是任意數字，boolean 是是非"},` +
	`"defaultValue":{"type":"number"}` +
	`},"required":["name","kind"],"additionalProperties":false}}`
