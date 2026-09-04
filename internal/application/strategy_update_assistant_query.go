package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// StrategyUpdateAssistantQuery lets the assistant rewrite a saved algorithm.
//
// A rewrite replaces everything the strategy remembers, so the assistant is told to
// read the strategy first: sending only the field it meant to change would blank the
// rest. That is a property of rewriting, not something this capability chose, and
// hiding it by merging silently would make "change the lookback to thirty" quietly
// throw away the algorithm.
type StrategyUpdateAssistantQuery struct {
	strategyApplication *StrategyApplication
}

func NewStrategyUpdateAssistantQuery(strategyApplication *StrategyApplication) *StrategyUpdateAssistantQuery {
	return &StrategyUpdateAssistantQuery{strategyApplication: strategyApplication}
}

func (strategyUpdateAssistantQuery *StrategyUpdateAssistantQuery) Name() string {
	return "update_strategy"
}

func (strategyUpdateAssistantQuery *StrategyUpdateAssistantQuery) Description() string {
	return "改寫一支既有策略。這是整包覆蓋：沒送的欄位會被清掉，" +
		"所以請先用 get_strategy 讀回來，改你要改的，其餘原樣送回。改回自己原本的名稱不算重複。"
}

func (strategyUpdateAssistantQuery *StrategyUpdateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"strategyId":{"type":"integer","description":"要改寫的策略識別碼"},` +
		strategyWriteArgumentSchema +
		`},"required":["strategyId","name","script"],"additionalProperties":false}`
}

// Run rewrites the strategy and hands it back as it now stands.
func (strategyUpdateAssistantQuery *StrategyUpdateAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	writeArguments := strategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyDto, updateError := strategyUpdateAssistantQuery.strategyApplication.UpdateStrategy(
		executionContext, writeArguments.ToWriteDto(writeArguments.StrategyID))
	if updateError != nil {
		return "", updateError
	}

	return renderedStrategy(strategyDto)
}
