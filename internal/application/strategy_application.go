package application

import (
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
	writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.CreateStrategy(writeDto)
}

func (strategyApplication *StrategyApplication) GetStrategy(id uint) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.GetStrategy(id)
}

func (strategyApplication *StrategyApplication) ListStrategies() ([]dto.StrategyDto, error) {
	return strategyApplication.strategyService.ListStrategies()
}

func (strategyApplication *StrategyApplication) UpdateStrategy(
	writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	return strategyApplication.strategyService.UpdateStrategy(writeDto)
}

func (strategyApplication *StrategyApplication) DeleteStrategy(id uint) error {
	return strategyApplication.strategyService.DeleteStrategy(id)
}
