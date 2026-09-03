package service

import (
	"context"

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
}

func NewStrategyService(strategyRepository domaininterface.IStrategyRepository) *StrategyService {
	return &StrategyService{strategyRepository: strategyRepository}
}

// CreateStrategy saves a new strategy and hands it back as stored. A strategy that
// breaks a rule is refused before anything is written.
func (strategyService *StrategyService) CreateStrategy(
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	savedStrategy, saveError := strategyService.strategyRepository.Save(executionContext, strategyDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyDto{}, saveError
	}

	return savedStrategy.ToDto(), nil
}

// GetStrategy returns the strategy carrying this identifier.
func (strategyService *StrategyService) GetStrategy(
	executionContext context.Context, id uint,
) (dto.StrategyDto, error) {
	strategy, findError := strategyService.strategyRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyDto{}, findError
	}

	return strategy.ToDto(), nil
}

// ListStrategies returns every saved strategy, ordered by name. Holding none is an
// answer rather than a failure.
func (strategyService *StrategyService) ListStrategies(executionContext context.Context) ([]dto.StrategyDto, error) {
	strategies, findError := strategyService.strategyRepository.FindAll(executionContext)
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
	executionContext context.Context, writeDto dto.StrategyWriteDto,
) (dto.StrategyDto, error) {
	// No strategy carries no identifier, so there is nothing here to rewrite. Saying
	// so here rather than letting the write go out is the difference between a
	// guarantee this code makes and one it borrows: a rewrite with no identifier
	// names no row, and what an ORM does with a write that names no row is its own
	// decision to change.
	if writeDto.ID == 0 {
		return dto.StrategyDto{}, domains.StrategyNotFound(writeDto.ID)
	}

	// Whether the strategy is there is settled before its content is judged. The
	// other way round, rewriting a strategy that does not exist with content that is
	// also wrong answers "a strategy must carry a name" — and supplying one still
	// leaves nothing to rewrite. The two refusals are different problems and must
	// not be handed over as one.
	if _, findError := strategyService.strategyRepository.FindOne(
		executionContext, writeDto.ID); findError != nil {
		return dto.StrategyDto{}, findError
	}

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)
	if validationError != nil {
		return dto.StrategyDto{}, validationError
	}

	updatedStrategy, updateError := strategyService.strategyRepository.Update(
		executionContext, strategyDomain.ToEntity())
	if updateError != nil {
		return dto.StrategyDto{}, updateError
	}

	return updatedStrategy.ToDto(), nil
}

// DeleteStrategy removes the strategy carrying this identifier for good.
func (strategyService *StrategyService) DeleteStrategy(
	executionContext context.Context, id uint,
) error {
	return strategyService.strategyRepository.Delete(executionContext, id)
}
