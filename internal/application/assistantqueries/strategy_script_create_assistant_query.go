package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// StrategyScriptCreateAssistantQuery lets the assistant save a new algorithm; deleting is
// deliberately not offered, since a mistaken delete loses work that cannot be recovered.
type StrategyScriptCreateAssistantQuery struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptCreateAssistantQuery(strategyScriptApplication *application.StrategyScriptApplication) *StrategyScriptCreateAssistantQuery {
	return &StrategyScriptCreateAssistantQuery{strategyScriptApplication: strategyScriptApplication}
}

func (strategyScriptCreateAssistantQuery *StrategyScriptCreateAssistantQuery) Name() string {
	return "create_strategy_script"
}

func (strategyScriptCreateAssistantQuery *StrategyScriptCreateAssistantQuery) Description() string {
	return "存下一支新的策略腳本（名稱＋指標算式＋指標值種類＋參數）。" +
		"名稱不得與既有策略腳本重複。存起來不代表跑起來——算式對不對要等真的拿去算才知道。"
}

func (strategyScriptCreateAssistantQuery *StrategyScriptCreateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + strategyScriptWriteArgumentSchema +
		`},"required":["name","script"],"additionalProperties":false}`
}

func (strategyScriptCreateAssistantQuery *StrategyScriptCreateAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	writeArguments := strategyScriptWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyScriptDto, createError := strategyScriptCreateAssistantQuery.strategyScriptApplication.CreateStrategyScript(
		executionContext, writeArguments.ToWriteDto(0, viewerID))
	if createError != nil {
		return "", createError
	}

	return renderedStrategyScript(strategyScriptDto)
}
