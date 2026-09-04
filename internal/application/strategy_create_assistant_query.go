package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// StrategyCreateAssistantQuery lets the assistant save a new algorithm.
//
// Saving is offered and deleting is not, and the asymmetry is the point: a strategy
// saved by mistake costs a name, while one deleted by mistake costs an algorithm that
// took several sittings to get right and cannot be recovered.
//
// Every rule a person's own save obeys is obeyed here — the name must be there, be
// short enough and be free — because they arrive at the same model. A refusal is
// handed back to the assistant as the reason, which it relays.
type StrategyCreateAssistantQuery struct {
	strategyApplication *StrategyApplication
}

func NewStrategyCreateAssistantQuery(strategyApplication *StrategyApplication) *StrategyCreateAssistantQuery {
	return &StrategyCreateAssistantQuery{strategyApplication: strategyApplication}
}

func (strategyCreateAssistantQuery *StrategyCreateAssistantQuery) Name() string {
	return "create_strategy"
}

func (strategyCreateAssistantQuery *StrategyCreateAssistantQuery) Description() string {
	return "存下一支新的策略（名稱＋指標算式＋指標值種類＋參數）。" +
		"名稱不得與既有策略重複。存起來不代表跑起來——算式對不對要等真的拿去算才知道。"
}

func (strategyCreateAssistantQuery *StrategyCreateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + strategyWriteArgumentSchema +
		`},"required":["name","script"],"additionalProperties":false}`
}

// Run saves the strategy and hands it back as stored.
func (strategyCreateAssistantQuery *StrategyCreateAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	writeArguments := strategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	strategyDto, createError := strategyCreateAssistantQuery.strategyApplication.CreateStrategy(
		executionContext, writeArguments.ToWriteDto(0))
	if createError != nil {
		return "", createError
	}

	return renderedStrategy(strategyDto)
}
