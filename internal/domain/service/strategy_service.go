package service

import (
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyService is the application layer's only entry point for saved strategies.
// Its public use-case methods never call one another.
//
// It is given no way to reach a K candle, and that is the point: saving a strategy
// must not read the market or work anything out, and a dependency that is not there
// cannot be used by accident later.
type StrategyService struct {
	strategyRepository domaininterface.IStrategyRepository
	maxCandleCount     int
}

func NewStrategyService(
	strategyRepository domaininterface.IStrategyRepository,
	maxCandleCount int,
) *StrategyService {
	return &StrategyService{
		strategyRepository: strategyRepository,
		maxCandleCount:     maxCandleCount,
	}
}

// CreateStrategy saves a new strategy and hands it back as stored. A strategy that
// breaks a rule is refused before anything is written.
func (strategyService *StrategyService) CreateStrategy(
	writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	strategyDomain, validationError := domains.NewStrategyDomain(
		writeDto, strategyService.maxCandleCount)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	savedStrategy, saveError := strategyService.strategyRepository.Save(strategyDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyDto{}, saveError
	}

	return savedStrategy.ToDto(), nil
}

// GetStrategy returns the strategy carrying this identifier.
func (strategyService *StrategyService) GetStrategy(id uint) (dto.StrategyDto, error) {
	strategy, findError := strategyService.strategyRepository.FindOne(id)
	if findError != nil {
		return dto.StrategyDto{}, findError
	}

	return strategy.ToDto(), nil
}

// ListStrategies returns every saved strategy, ordered by name. Holding none is an
// answer rather than a failure.
func (strategyService *StrategyService) ListStrategies() ([]dto.StrategyDto, error) {
	strategies, findError := strategyService.strategyRepository.FindAll()
	if findError != nil {
		return nil, findError
	}

	strategyDtos := make([]dto.StrategyDto, 0, len(strategies))
	for _, strategy := range strategies {
		strategyDtos = append(strategyDtos, strategy.ToDto())
	}

	return strategyDtos, nil
}

// UpdateStrategy rewrites the strategy this write names and hands it back as it now
// stands. Every rule that governs a new strategy governs a rewritten one, because
// both arrive here as the same shape and are judged by the same model.
func (strategyService *StrategyService) UpdateStrategy(
	writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	strategyDomain, validationError := domains.NewStrategyDomain(
		writeDto, strategyService.maxCandleCount)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	updatedStrategy, updateError := strategyService.strategyRepository.Update(
		strategyDomain.ToEntity())
	if updateError != nil {
		return dto.StrategyDto{}, updateError
	}

	return updatedStrategy.ToDto(), nil
}

// DeleteStrategy removes the strategy carrying this identifier for good.
func (strategyService *StrategyService) DeleteStrategy(id uint) error {
	return strategyService.strategyRepository.Delete(id)
}
