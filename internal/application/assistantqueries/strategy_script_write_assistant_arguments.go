package assistantqueries

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type strategyScriptParameterAssistantArgument struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// strategyScriptWriteAssistantArguments serves both save and rewrite, since a rewrite replaces
// every field; a zero identifier means the script is new.
type strategyScriptWriteAssistantArguments struct {
	StrategyScriptID uint                                       `json:"strategyScriptId"`
	Name             string                                     `json:"name"`
	Description      string                                     `json:"description"`
	Script           string                                     `json:"script"`
	ResultType       string                                     `json:"resultType"`
	Parameters       []strategyScriptParameterAssistantArgument `json:"parameters"`
}

// ToWriteDto takes the identity and owner from the calling capability, never from the assistant,
// which always acts for whoever asked it.
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

// strategyScriptWriteArgumentSchema is shared by both writing capabilities, which differ only in
// whether the identifier is required.
const strategyScriptWriteArgumentSchema = `` +
	`"description":{"type":"string","description":"這支策略腳本在做什麼，發佈到市集時別人只看得到這一段"},` +
	`"name":{"type":"string","description":"策略腳本名稱，不得空白、不得與既有策略腳本重複，上限 128 字"},` +
	`"script":{"type":"string","description":"指標算式（Go 函式本文），不得空白"},` +
	`"resultType":{"type":"string","enum":["float","floatList","bool","boolList","signal"],` +
	`"description":"指標值種類，未給視為 float。要給交易策略當信號來源的腳本一律用 signal"},` +
	`"parameters":{"type":"array","description":"這支策略腳本自己的參數","items":{"type":"object","properties":{` +
	`"description":{"type":"string","description":"這支策略腳本在做什麼，發佈到市集時別人只看得到這一段"},` +
	`"name":{"type":"string"},` +
	`"kind":{"type":"string","enum":["lookbackCount","number","boolean"],"description":"lookbackCount 是要看過去幾根，number 是任意數字，boolean 是是非"},` +
	`"defaultValue":{"type":"number"}` +
	`},"required":["name","kind"],"additionalProperties":false}}`
