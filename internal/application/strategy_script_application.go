package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyScriptApplication orchestrates the saved strategy script use cases.
//
// Every one of them names who is asking. That is not plumbing: which strategy scripts
// exist, for the purposes of an answer, depends on who is asking, so a use case
// that did not know would have to guess.
type StrategyScriptApplication struct {
	strategyScriptService *service.StrategyScriptService
}

func NewStrategyScriptApplication(strategyScriptService *service.StrategyScriptService) *StrategyScriptApplication {
	return &StrategyScriptApplication{strategyScriptService: strategyScriptService}
}

func (strategyScriptApplication *StrategyScriptApplication) CreateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.CreateStrategyScript(executionContext, writeDto)
}

func (strategyScriptApplication *StrategyScriptApplication) GetStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.GetStrategyScript(executionContext, viewerID, id)
}

func (strategyScriptApplication *StrategyScriptApplication) ListAvailableStrategyScripts(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategyScriptsDto, error) {
	return strategyScriptApplication.strategyScriptService.ListAvailableStrategyScripts(executionContext, viewerID)
}

func (strategyScriptApplication *StrategyScriptApplication) UpdateStrategyScript(
	executionContext context.Context, writeDto dto.StrategyScriptWriteDto,
) (dto.StrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.UpdateStrategyScript(executionContext, writeDto)
}

func (strategyScriptApplication *StrategyScriptApplication) DeleteStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return strategyScriptApplication.strategyScriptService.DeleteStrategyScript(executionContext, viewerID, id)
}

// ResolveRunnableStrategyScript hands back the algorithm behind an identifier, for
// whoever may run it.
//
// It is exported because running is orchestrated one layer up — a calculation is a
// strategy script service answer followed by a calculation service answer, and a domain
// service does not call another domain service. What it returns never reaches a
// controller.
func (strategyScriptApplication *StrategyScriptApplication) ResolveRunnableStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.ResolveRunnableStrategyScript(executionContext, viewerID, id)
}
