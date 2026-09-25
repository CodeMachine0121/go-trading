package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyScriptUpdateAssistantQuery lets the assistant rewrite a saved algorithm; a rewrite
// replaces every field, so the assistant is told to read the script first rather than merged silently.
type StrategyScriptUpdateAssistantQuery struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptUpdateAssistantQuery(strategyScriptApplication *application.StrategyScriptApplication) *StrategyScriptUpdateAssistantQuery {
	return &StrategyScriptUpdateAssistantQuery{strategyScriptApplication: strategyScriptApplication}
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) Name() string {
	return "update_strategy_script"
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) Description() string {
	return "改寫一支既有策略腳本，包含它的參數——新增、改名、改種類、改預設值、移除某個參數都在這裡做。" +
		"這是整包覆蓋：沒送的欄位與沒列進 parameters 的參數都會被清掉，" +
		"所以請先用 get_strategy_script 讀回來，改你要改的，其餘（含每個要保留的參數）原樣送回。改回自己原本的名稱不算重複。"
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"strategyScriptId":{"type":"integer","description":"要改寫的策略腳本識別碼"},` +
		strategyScriptWriteArgumentSchema +
		`},"required":["strategyScriptId","name","script"],"additionalProperties":false}`
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	writeArguments := strategyScriptWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyScriptDto, updateError := strategyScriptUpdateAssistantQuery.strategyScriptApplication.UpdateStrategyScript(
		executionContext, writeArguments.ToWriteDto(writeArguments.StrategyScriptID, origin.ViewerID))
	if updateError != nil {
		return "", updateError
	}

	return renderedStrategyScript(strategyScriptDto)
}
