package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// StrategyScriptListAssistantQuery lets the assistant see which algorithms are already
// saved.
//
// It hands over each strategy script's identifier, name and the shape of value it produces,
// but not the algorithm itself. A list is for choosing from, and a script is the
// longest thing a strategy script holds — sending every script every time the assistant
// wants to know what exists would be the single most expensive habit it could form.
type StrategyScriptListAssistantQuery struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptListAssistantQuery(strategyScriptApplication *application.StrategyScriptApplication) *StrategyScriptListAssistantQuery {
	return &StrategyScriptListAssistantQuery{strategyScriptApplication: strategyScriptApplication}
}

func (strategyScriptListAssistantQuery *StrategyScriptListAssistantQuery) Name() string {
	return "list_strategy_scripts"
}

func (strategyScriptListAssistantQuery *StrategyScriptListAssistantQuery) Description() string {
	return "列出已存的每一支策略腳本：識別碼、名稱、指標值種類與參數。" +
		"不含算式本文——要看算式請用 get_strategy_script 指名一支。"
}

func (strategyScriptListAssistantQuery *StrategyScriptListAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{},"additionalProperties":false}`
}

// strategyScriptDigest is a strategy script as it appears in a list: enough to pick one by,
// without the algorithm itself.
type strategyScriptDigest struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	ResultType     string   `json:"resultType"`
	ParameterNames []string `json:"parameterNames"`
	// Mine says whether the asker owns this one. Only their own can be rewritten,
	// and saying so here is what keeps the assistant from offering to.
	Mine bool `json:"mine"`
}

// Run hands over, in brief, everything the person who asked can pick from: their
// own strategy scripts and the ones they took off the marketplace. Holding none is an
// answer, not a refusal.
//
// The two arrive as one list here, unlike over HTTP, and that costs nothing: a
// digest never carried an algorithm to begin with, so there is no shape difference
// left to preserve. What it does carry is who each one belongs to, so the assistant
// does not offer to rewrite one that is not the asker's.
func (strategyScriptListAssistantQuery *StrategyScriptListAssistantQuery) Run(
	executionContext context.Context, viewerID uint, _ string,
) (string, error) {
	availableStrategyScriptsDto, listError := strategyScriptListAssistantQuery.strategyScriptApplication.ListAvailableStrategyScripts(
		executionContext, viewerID)
	if listError != nil {
		return "", listError
	}

	digests := make([]strategyScriptDigest, 0,
		len(availableStrategyScriptsDto.Mine)+len(availableStrategyScriptsDto.Adopted))
	for _, strategyScriptDto := range availableStrategyScriptsDto.Mine {
		parameterNames := make([]string, 0, len(strategyScriptDto.Parameters))
		for _, parameterDto := range strategyScriptDto.Parameters {
			parameterNames = append(parameterNames, parameterDto.Name)
		}

		digests = append(digests, strategyScriptDigest{
			ID:             strategyScriptDto.ID,
			Name:           strategyScriptDto.Name,
			ResultType:     strategyScriptDto.ResultType,
			ParameterNames: parameterNames,
			Mine:           true,
		})
	}

	for _, publishedStrategyScriptDto := range availableStrategyScriptsDto.Adopted {
		parameterNames := make([]string, 0, len(publishedStrategyScriptDto.Parameters))
		for _, parameterDto := range publishedStrategyScriptDto.Parameters {
			parameterNames = append(parameterNames, parameterDto.Name)
		}

		digests = append(digests, strategyScriptDigest{
			ID:             publishedStrategyScriptDto.ID,
			Name:           publishedStrategyScriptDto.Name,
			ResultType:     publishedStrategyScriptDto.ResultType,
			ParameterNames: parameterNames,
			Mine:           false,
		})
	}

	payload, marshalError := json.Marshal(struct {
		StrategyScripts []strategyScriptDigest `json:"strategyScripts"`
	}{StrategyScripts: digests})
	if marshalError != nil {
		return "", fmt.Errorf("render strategy scripts: %w", marshalError)
	}

	return string(payload), nil
}
