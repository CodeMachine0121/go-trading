package application

import (
	"context"
	"encoding/json"
	"fmt"
)

// StrategyListAssistantQuery lets the assistant see which algorithms are already
// saved.
//
// It hands over each strategy's identifier, name and the shape of value it produces,
// but not the algorithm itself. A list is for choosing from, and a script is the
// longest thing a strategy holds — sending every script every time the assistant
// wants to know what exists would be the single most expensive habit it could form.
type StrategyListAssistantQuery struct {
	strategyApplication *StrategyApplication
}

func NewStrategyListAssistantQuery(strategyApplication *StrategyApplication) *StrategyListAssistantQuery {
	return &StrategyListAssistantQuery{strategyApplication: strategyApplication}
}

func (strategyListAssistantQuery *StrategyListAssistantQuery) Name() string {
	return "list_strategies"
}

func (strategyListAssistantQuery *StrategyListAssistantQuery) Description() string {
	return "列出已存的每一支策略：識別碼、名稱、指標值種類與參數。" +
		"不含算式本文——要看算式請用 get_strategy 指名一支。"
}

func (strategyListAssistantQuery *StrategyListAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{},"additionalProperties":false}`
}

// strategyDigest is a strategy as it appears in a list: enough to pick one by,
// without the algorithm itself.
type strategyDigest struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	ResultType     string   `json:"resultType"`
	ParameterNames []string `json:"parameterNames"`
}

// Run hands over every saved strategy in brief. Holding none is an answer, not a
// refusal.
func (strategyListAssistantQuery *StrategyListAssistantQuery) Run(
	executionContext context.Context, _ string,
) (string, error) {
	strategyDtos, listError := strategyListAssistantQuery.strategyApplication.ListStrategies(executionContext)
	if listError != nil {
		return "", listError
	}

	digests := make([]strategyDigest, 0, len(strategyDtos))
	for _, strategyDto := range strategyDtos {
		parameterNames := make([]string, 0, len(strategyDto.Parameters))
		for _, parameterDto := range strategyDto.Parameters {
			parameterNames = append(parameterNames, parameterDto.Name)
		}

		digests = append(digests, strategyDigest{
			ID:             strategyDto.ID,
			Name:           strategyDto.Name,
			ResultType:     strategyDto.ResultType,
			ParameterNames: parameterNames,
		})
	}

	payload, marshalError := json.Marshal(struct {
		Strategies []strategyDigest `json:"strategies"`
	}{Strategies: digests})
	if marshalError != nil {
		return "", fmt.Errorf("render strategies: %w", marshalError)
	}

	return string(payload), nil
}
