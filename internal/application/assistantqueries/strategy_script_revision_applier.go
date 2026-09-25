package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyScriptRevisionApplier reads a rewrite in the shape update_strategy_script takes and carries it out
// through the same rewrite a person makes.
type StrategyScriptRevisionApplier struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptRevisionApplier(
	strategyScriptApplication *application.StrategyScriptApplication,
) *StrategyScriptRevisionApplier {
	return &StrategyScriptRevisionApplier{strategyScriptApplication: strategyScriptApplication}
}

func (strategyScriptRevisionApplier *StrategyScriptRevisionApplier) SubjectKind() vo.AssistantRevisionSubjectKindVo {
	return vo.AssistantRevisionSubjectStrategyScript
}

func (strategyScriptRevisionApplier *StrategyScriptRevisionApplier) Inspect(
	executionContext context.Context, viewerID uint, content string,
) (dto.RewriteTargetDto, error) {
	writeDto, readError := strategyScriptRevisionApplier.writeDtoOf(viewerID, content)
	if readError != nil {
		return dto.RewriteTargetDto{}, readError
	}

	return strategyScriptRevisionApplier.strategyScriptApplication.InspectStrategyScriptRewrite(
		executionContext, writeDto)
}

func (strategyScriptRevisionApplier *StrategyScriptRevisionApplier) Apply(
	executionContext context.Context, viewerID uint, content string,
) (string, error) {
	writeDto, readError := strategyScriptRevisionApplier.writeDtoOf(viewerID, content)
	if readError != nil {
		return "", readError
	}

	strategyScriptDto, updateError := strategyScriptRevisionApplier.strategyScriptApplication.UpdateStrategyScript(
		executionContext, writeDto)
	if updateError != nil {
		return "", updateError
	}

	return renderedStrategyScript(strategyScriptDto)
}

func (strategyScriptRevisionApplier *StrategyScriptRevisionApplier) writeDtoOf(
	viewerID uint, content string,
) (dto.StrategyScriptWriteDto, error) {
	// Unknown fields are refused rather than dropped, so what the owner reviews is exactly what gets written.
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	writeArguments := strategyScriptWriteAssistantArguments{}
	if unmarshalError := decoder.Decode(&writeArguments); unmarshalError != nil {
		return dto.StrategyScriptWriteDto{}, fmt.Errorf(
			"%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	return writeArguments.ToWriteDto(writeArguments.StrategyScriptID, viewerID), nil
}
