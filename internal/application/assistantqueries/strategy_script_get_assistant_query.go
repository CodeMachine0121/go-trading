package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type strategyScriptGetAssistantArguments struct {
	StrategyScriptID uint `json:"strategyScriptId"`
}

// StrategyScriptGetAssistantQuery hands the assistant a saved strategy script in full, because a
// rewrite replaces everything and the assistant must send the unchanged parts back.
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

// renderedStrategyScript is the one shape reading, saving and rewriting all hand back.
func renderedStrategyScript(strategyScriptDto dto.StrategyScriptDto) (string, error) {
	payload, marshalError := json.Marshal(strategyScriptDto)
	if marshalError != nil {
		return "", fmt.Errorf("render strategy script: %w", marshalError)
	}

	return string(payload), nil
}
