package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// StrategyScriptListAssistantQuery lists saved strategy scripts without their algorithms, since
// sending every script on each listing would be the assistant's most expensive habit.
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

type strategyScriptDigest struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	ResultType     string   `json:"resultType"`
	ParameterNames []string `json:"parameterNames"`
	// Mine keeps the assistant from offering to rewrite a script the asker does not own.
	Mine bool `json:"mine"`
}

// Run lists the asker's own strategy scripts and those adopted from the marketplace as one list;
// holding none is an answer, not a refusal.
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
