package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// StrategyListAssistantQuery lets the assistant see which algorithms are already
// saved.
//
// It hands over each strategy's identifier, name and the shape of value it produces,
// but not the algorithm itself. A list is for choosing from, and a script is the
// longest thing a strategy holds — sending every script every time the assistant
// wants to know what exists would be the single most expensive habit it could form.
type StrategyListAssistantQuery struct {
	strategyApplication *application.StrategyApplication
}

func NewStrategyListAssistantQuery(strategyApplication *application.StrategyApplication) *StrategyListAssistantQuery {
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
	// Mine says whether the asker owns this one. Only their own can be rewritten,
	// and saying so here is what keeps the assistant from offering to.
	Mine bool `json:"mine"`
}

// Run hands over, in brief, everything the person who asked can pick from: their
// own strategies and the ones they took off the marketplace. Holding none is an
// answer, not a refusal.
//
// The two arrive as one list here, unlike over HTTP, and that costs nothing: a
// digest never carried an algorithm to begin with, so there is no shape difference
// left to preserve. What it does carry is who each one belongs to, so the assistant
// does not offer to rewrite one that is not the asker's.
func (strategyListAssistantQuery *StrategyListAssistantQuery) Run(
	executionContext context.Context, viewerID uint, _ string,
) (string, error) {
	availableStrategiesDto, listError := strategyListAssistantQuery.strategyApplication.ListAvailableStrategies(
		executionContext, viewerID)
	if listError != nil {
		return "", listError
	}

	digests := make([]strategyDigest, 0,
		len(availableStrategiesDto.Mine)+len(availableStrategiesDto.Adopted))
	for _, strategyDto := range availableStrategiesDto.Mine {
		parameterNames := make([]string, 0, len(strategyDto.Parameters))
		for _, parameterDto := range strategyDto.Parameters {
			parameterNames = append(parameterNames, parameterDto.Name)
		}

		digests = append(digests, strategyDigest{
			ID:             strategyDto.ID,
			Name:           strategyDto.Name,
			ResultType:     strategyDto.ResultType,
			ParameterNames: parameterNames,
			Mine:           true,
		})
	}

	for _, publishedStrategyDto := range availableStrategiesDto.Adopted {
		parameterNames := make([]string, 0, len(publishedStrategyDto.Parameters))
		for _, parameterDto := range publishedStrategyDto.Parameters {
			parameterNames = append(parameterNames, parameterDto.Name)
		}

		digests = append(digests, strategyDigest{
			ID:             publishedStrategyDto.ID,
			Name:           publishedStrategyDto.Name,
			ResultType:     publishedStrategyDto.ResultType,
			ParameterNames: parameterNames,
			Mine:           false,
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
