package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// strategyScriptGetAssistantArguments is what the assistant sends to read one strategy script.
type strategyScriptGetAssistantArguments struct {
	StrategyScriptID uint `json:"strategyScriptId"`
}

// StrategyScriptGetAssistantQuery lets the assistant read one saved strategy script in full,
// algorithm included.
//
// Reading it in full is what makes changing it possible: a rewrite replaces
// everything a strategy script remembers, so an assistant asked to change one knob has to
// know the rest before it can send them back unchanged.
type StrategyScriptGetAssistantQuery struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptGetAssistantQuery(strategyScriptApplication *application.StrategyScriptApplication) *StrategyScriptGetAssistantQuery {
	return &StrategyScriptGetAssistantQuery{strategyScriptApplication: strategyScriptApplication}
}

func (strategyScriptGetAssistantQuery *StrategyScriptGetAssistantQuery) Name() string {
	return "get_strategy_script"
}

func (strategyScriptGetAssistantQuery *StrategyScriptGetAssistantQuery) Description() string {
	return "以識別碼讀一支策略腳本的完整內容，含指標算式與每個參數（名稱、種類、預設值）。" +
		"要修改一支策略腳本——含新增／修改／移除它的參數——前一定要先讀，因為 update_strategy_script 是整包覆蓋。"
}

func (strategyScriptGetAssistantQuery *StrategyScriptGetAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"strategyScriptId":{"type":"integer","description":"策略腳本識別碼"}` +
		`},"required":["strategyScriptId"],"additionalProperties":false}`
}

// Run hands over the strategy script in full. A strategy script that is not there comes back as the
// system's own words for that, which the assistant relays rather than reinvents.
func (strategyScriptGetAssistantQuery *StrategyScriptGetAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	getArguments := strategyScriptGetAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &getArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyScriptDto, findError := strategyScriptGetAssistantQuery.strategyScriptApplication.GetStrategyScript(
		executionContext, viewerID, getArguments.StrategyScriptID)
	if findError != nil {
		return "", findError
	}

	return renderedStrategyScript(strategyScriptDto)
}

// renderedStrategyScript is one strategy script as the assistant reads it. Reading one, saving one
// and rewriting one all hand back the same shape, so the assistant never has to learn
// two ways of looking at the same thing.
func renderedStrategyScript(strategyScriptDto dto.StrategyScriptDto) (string, error) {
	payload, marshalError := json.Marshal(strategyScriptDto)
	if marshalError != nil {
		return "", fmt.Errorf("render strategy script: %w", marshalError)
	}

	return string(payload), nil
}
