package assistantqueries

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// strategy scriptParameterAssistantArgument is one knob as the assistant declares it.
type strategyScriptParameterAssistantArgument struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// strategy scriptWriteAssistantArguments is what the assistant sends to save or rewrite a
// strategy script.
//
// One shape serves both, exactly as it does for a person: a rewrite replaces
// everything a strategy script remembers, so there is no field the assistant may set on one
// path and not the other. Which strategy script is meant is the identifier, and a zero one
// means none yet.
type strategyScriptWriteAssistantArguments struct {
	StrategyScriptID uint                                       `json:"strategyScriptId"`
	Name             string                                     `json:"name"`
	Description      string                                     `json:"description"`
	Script           string                                     `json:"script"`
	ResultType       string                                     `json:"resultType"`
	Parameters       []strategyScriptParameterAssistantArgument `json:"parameters"`
}

// ToWriteDto turns what the assistant declared into the shape the domain judges,
// taking the identity and the owner from the arguments so that the capability
// calling this decides whether a strategy script is being saved or rewritten, and on whose
// behalf. The assistant never names an owner itself — it acts for whoever asked it.
func (strategyScriptWriteAssistantArguments strategyScriptWriteAssistantArguments) ToWriteDto(id uint, ownerID uint) dto.StrategyScriptWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(strategyScriptWriteAssistantArguments.Parameters))
	for _, parameter := range strategyScriptWriteAssistantArguments.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, dto.StrategyScriptParameterWriteDto{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			DefaultValue: parameter.DefaultValue,
		})
	}

	return dto.StrategyScriptWriteDto{
		ID:          id,
		OwnerID:     ownerID,
		Name:        strategyScriptWriteAssistantArguments.Name,
		Description: strategyScriptWriteAssistantArguments.Description,
		Script:      strategyScriptWriteAssistantArguments.Script,
		ResultType:  strategyScriptWriteAssistantArguments.ResultType,
		Parameters:  parameterWriteDtos,
	}
}

// strategy scriptWriteArgumentSchema is the arguments both writing capabilities take. It is
// written once because they take the same ones — the only difference is whether the
// identifier is required, and each says that for itself.
const strategyScriptWriteArgumentSchema = `` +
	`"description":{"type":"string","description":"這支策略腳本在做什麼，發佈到市集時別人只看得到這一段"},` +
	`"name":{"type":"string","description":"策略腳本名稱，不得空白、不得與既有策略腳本重複，上限 128 字"},` +
	`"script":{"type":"string","description":"指標算式（Go 函式本文），不得空白"},` +
	`"resultType":{"type":"string","enum":["float","floatList","bool","boolList"],"description":"指標值種類，未給視為 float"},` +
	`"parameters":{"type":"array","description":"這支策略腳本自己的參數","items":{"type":"object","properties":{` +
	`"description":{"type":"string","description":"這支策略腳本在做什麼，發佈到市集時別人只看得到這一段"},` +
	`"name":{"type":"string"},` +
	`"kind":{"type":"string","enum":["lookbackCount","number","boolean"],"description":"lookbackCount 是要看過去幾根，number 是任意數字，boolean 是是非"},` +
	`"defaultValue":{"type":"number"}` +
	`},"required":["name","kind"],"additionalProperties":false}}`
