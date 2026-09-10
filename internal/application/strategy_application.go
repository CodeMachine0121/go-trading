package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyApplication orchestrates the saved strategy use cases.
//
// Every one of them names who is asking. That is not plumbing: which strategies
// exist, for the purposes of an answer, depends on who is asking, so a use case
// that did not know would have to guess.
type StrategyApplication struct {
	strategyService *service.StrategyService
}

func NewStrategyApplication(strategyService *service.StrategyService) *StrategyApplication {
	return &StrategyApplication{strategyService: strategyService}
}

func (strategyApplication *StrategyApplication) CreateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.CreateStrategy(executionContext, writeDto)
}

func (strategyApplication *StrategyApplication) GetStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.GetStrategy(executionContext, viewerID, id)
}

func (strategyApplication *StrategyApplication) ListAvailableStrategies(
	executionContext context.Context, viewerID uint,
) (dto.AvailableStrategiesDto, error) {
	return strategyApplication.strategyService.ListAvailableStrategies(executionContext, viewerID)
}

func (strategyApplication *StrategyApplication) UpdateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.UpdateStrategy(executionContext, writeDto)
}

func (strategyApplication *StrategyApplication) DeleteStrategy(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return strategyApplication.strategyService.DeleteStrategy(executionContext, viewerID, id)
}

// ResolveRunnableStrategy hands back the algorithm behind an identifier, for
// whoever may run it.
//
// It is exported because running is orchestrated one layer up — a calculation is a
// strategy service answer followed by a calculation service answer, and a domain
// service does not call another domain service. What it returns never reaches a
// controller.
func (strategyApplication *StrategyApplication) ResolveRunnableStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.RunnableStrategyDto, error) {
	return strategyApplication.strategyService.ResolveRunnableStrategy(executionContext, viewerID, id)
}
