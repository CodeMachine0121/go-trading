package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

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

// ResolveRunnableStrategyScript returns the algorithm behind an identifier for whoever may run it; it is exported for cross-service orchestration and its result never reaches a controller.
func (strategyScriptApplication *StrategyScriptApplication) ResolveRunnableStrategyScript(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyScriptDto, error) {
	return strategyScriptApplication.strategyScriptService.ResolveRunnableStrategyScript(executionContext, viewerID, id)
}
