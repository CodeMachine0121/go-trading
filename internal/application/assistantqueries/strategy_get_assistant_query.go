package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// strategyGetAssistantArguments is what the assistant sends to read one strategy.
type strategyGetAssistantArguments struct {
	StrategyID uint `json:"strategyId"`
}

// StrategyGetAssistantQuery lets the assistant read one saved strategy in full,
// algorithm included.
//
// Reading it in full is what makes changing it possible: a rewrite replaces
// everything a strategy remembers, so an assistant asked to change one knob has to
// know the rest before it can send them back unchanged.
type StrategyGetAssistantQuery struct {
	strategyApplication *application.StrategyApplication
}

func NewStrategyGetAssistantQuery(strategyApplication *application.StrategyApplication) *StrategyGetAssistantQuery {
	return &StrategyGetAssistantQuery{strategyApplication: strategyApplication}
}

func (strategyGetAssistantQuery *StrategyGetAssistantQuery) Name() string {
	return "get_strategy"
}

func (strategyGetAssistantQuery *StrategyGetAssistantQuery) Description() string {
	return "以識別碼讀一支策略的完整內容，含指標算式與每個參數（名稱、種類、預設值）。" +
		"要修改一支策略——含新增／修改／移除它的參數——前一定要先讀，因為 update_strategy 是整包覆蓋。"
}

func (strategyGetAssistantQuery *StrategyGetAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"strategyId":{"type":"integer","description":"策略識別碼"}` +
		`},"required":["strategyId"],"additionalProperties":false}`
}

// Run hands over the strategy in full. A strategy that is not there comes back as the
// system's own words for that, which the assistant relays rather than reinvents.
func (strategyGetAssistantQuery *StrategyGetAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	getArguments := strategyGetAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &getArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyDto, findError := strategyGetAssistantQuery.strategyApplication.GetStrategy(
		executionContext, getArguments.StrategyID)
	if findError != nil {
		return "", findError
	}

	return renderedStrategy(strategyDto)
}

// renderedStrategy is one strategy as the assistant reads it. Reading one, saving one
// and rewriting one all hand back the same shape, so the assistant never has to learn
// two ways of looking at the same thing.
func renderedStrategy(strategyDto dto.StrategyDto) (string, error) {
	payload, marshalError := json.Marshal(strategyDto)
	if marshalError != nil {
		return "", fmt.Errorf("render strategy: %w", marshalError)
	}

	return string(payload), nil
}
