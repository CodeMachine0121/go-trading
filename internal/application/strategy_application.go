package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyApplication orchestrates the saved strategy use cases.
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
	executionContext context.Context, id uint,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.GetStrategy(executionContext, id)
}

func (strategyApplication *StrategyApplication) ListStrategies(
	executionContext context.Context,
) ([]dto.StrategyDto, error) {
	return strategyApplication.strategyService.ListStrategies(executionContext)
}

func (strategyApplication *StrategyApplication) UpdateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.UpdateStrategy(executionContext, writeDto)
}

func (strategyApplication *StrategyApplication) DeleteStrategy(
	executionContext context.Context, id uint,
) error {
	return strategyApplication.strategyService.DeleteStrategy(executionContext, id)
}
