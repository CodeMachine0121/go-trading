package assistantqueries

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyScriptUpdateAssistantQuery lets the assistant rewrite a saved algorithm; a rewrite replaces every field,
// so the assistant is told to read the script first, and a script the person already had is only proposed.
type StrategyScriptUpdateAssistantQuery struct {
	assistantRevisionApplication *application.AssistantRevisionApplication
}

func NewStrategyScriptUpdateAssistantQuery(
	assistantRevisionApplication *application.AssistantRevisionApplication,
) *StrategyScriptUpdateAssistantQuery {
	return &StrategyScriptUpdateAssistantQuery{assistantRevisionApplication: assistantRevisionApplication}
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) Name() string {
	return "update_strategy_script"
}

func (strategyScriptUpdateAssistantQuery *StrategyScriptUpdateAssistantQuery) Description() string {
	return "改寫一支既有策略腳本，包含它的參數——新增、改名、改種類、改預設值、移除某個參數都在這裡做。" +
		"這是整包覆蓋：沒送的欄位與沒列進 parameters 的參數都會被清掉，" +
		"所以請先用 get_strategy_script 讀回來，改你要改的，其餘（含每個要保留的參數）原樣送回。改回自己原本的名稱不算重複。" +
		"**這段對話裡你自己剛建立、而且沒有任何機器人用到的**策略腳本會直接改好；" +
		"其餘的（使用者本來就有的、別段對話建立的、有機器人在用的）只會變成一筆「等使用者確認的修改」，" +
		"在使用者按下確認之前一個字都不會變——回傳會明講 pending，這時候要告訴使用者請他確認，不要說已經改好。" +
		"確認時如果有執行中的機器人在用這支腳本，確認會被擋下，使用者得先停掉它們。"
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
	return strategyScriptUpdateAssistantQuery.assistantRevisionApplication.Revise(
		executionContext, origin, vo.AssistantRevisionSubjectStrategyScript, arguments)
}
