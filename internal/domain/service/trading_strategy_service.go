package service

import (
	"context"
	"strings"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// It deliberately cannot reach scripts or bots; the application layer joins those checks in.
type TradingStrategyService struct {
	tradingStrategyRepository domaininterface.ITradingStrategyRepository
}

func NewTradingStrategyService(
	tradingStrategyRepository domaininterface.ITradingStrategyRepository,
) *TradingStrategyService {
	return &TradingStrategyService{tradingStrategyRepository: tradingStrategyRepository}
}

// CreateTradingStrategy saves a new trading strategy, refusing one that breaks a rule before anything is written.
func (tradingStrategyService *TradingStrategyService) CreateTradingStrategy(
	executionContext context.Context, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	// Clearing the ID stops a create from overwriting an existing (possibly foreign) strategy.
	writeDto.ID = 0

	tradingStrategyDomain, validationError := domains.NewTradingStrategyDomain(writeDto)
	if validationError != nil {
		return dto.TradingStrategyDto{}, validationError
	}

	savedTradingStrategy, saveError := tradingStrategyService.tradingStrategyRepository.Save(
		executionContext, tradingStrategyDomain.ToEntity())
	if saveError != nil {
		return dto.TradingStrategyDto{}, saveError
	}

	return savedTradingStrategy.ToDto(), nil
}

// ListTradingStrategies returns the owner's trading strategies by name; having none is an empty list.
func (tradingStrategyService *TradingStrategyService) ListTradingStrategies(
	executionContext context.Context, ownerID uint,
) ([]dto.TradingStrategyDto, error) {
	tradingStrategies, findError := tradingStrategyService.tradingStrategyRepository.FindAllByOwner(
		executionContext, ownerID)
	if findError != nil {
		return nil, findError
	}

	tradingStrategyDtos := make([]dto.TradingStrategyDto, 0, len(tradingStrategies))
	for _, tradingStrategy := range tradingStrategies {
		tradingStrategyDtos = append(tradingStrategyDtos, tradingStrategy.ToDto())
	}

	return tradingStrategyDtos, nil
}

// GetTradingStrategy returns the viewer's own trading strategy; bot rounds read it as the bot's owner, so a round runs exactly what the owner sees.
func (tradingStrategyService *TradingStrategyService) GetTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (dto.TradingStrategyDto, error) {
	tradingStrategy, findError := tradingStrategyService.findOwnedTradingStrategy(
		executionContext, viewerID, id)
	if findError != nil {
		return dto.TradingStrategyDto{}, findError
	}

	return tradingStrategy.ToDto(), nil
}

// UpdateTradingStrategy rewrites the viewer's strategy under the create rules; the caller checks for running bots.
func (tradingStrategyService *TradingStrategyService) UpdateTradingStrategy(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	_, tradingStrategyDomain, rewriteError := tradingStrategyService.preparedRewrite(
		executionContext, viewerID, writeDto)
	if rewriteError != nil {
		return dto.TradingStrategyDto{}, rewriteError
	}

	savedTradingStrategy, saveError := tradingStrategyService.tradingStrategyRepository.Save(
		executionContext, tradingStrategyDomain.ToEntity())
	if saveError != nil {
		return dto.TradingStrategyDto{}, saveError
	}

	return savedTradingStrategy.ToDto(), nil
}

// InspectTradingStrategyRewrite applies every rule a rewrite would, writes nothing, and returns the strategy as it stands.
func (tradingStrategyService *TradingStrategyService) InspectTradingStrategyRewrite(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	storedTradingStrategy, _, rewriteError := tradingStrategyService.preparedRewrite(
		executionContext, viewerID, writeDto)
	if rewriteError != nil {
		return dto.TradingStrategyDto{}, rewriteError
	}

	return storedTradingStrategy.ToDto(), nil
}

// preparedRewrite checks ownership before content, so a stranger's strategy reads as missing.
func (tradingStrategyService *TradingStrategyService) preparedRewrite(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (entities.TradingStrategy, domains.TradingStrategyDomain, error) {
	storedTradingStrategy, findError := tradingStrategyService.findOwnedTradingStrategy(
		executionContext, viewerID, writeDto.ID)
	if findError != nil {
		return entities.TradingStrategy{}, domains.TradingStrategyDomain{}, findError
	}

	// The owner always comes from storage, so a strategy cannot change hands.
	writeDto.OwnerID = storedTradingStrategy.OwnerID

	// Omitting the market data kind keeps the stored one; naming the other kind is refused.
	storedMarketDataKind, storedKindError := domains.NewMarketDataKindDomain(
		storedTradingStrategy.MarketDataKind)
	if storedKindError != nil {
		return entities.TradingStrategy{}, domains.TradingStrategyDomain{}, storedKindError
	}

	retainedMarketDataKind, retainError := storedMarketDataKind.RetainingForTradingStrategy(
		writeDto.MarketDataKind)
	if retainError != nil {
		return entities.TradingStrategy{}, domains.TradingStrategyDomain{}, retainError
	}
	writeDto.MarketDataKind = string(retainedMarketDataKind.Value())

	// Omitting the trading mode keeps the stored one, so a rename cannot turn a short-only strategy into one that also buys.
	if strings.TrimSpace(writeDto.TradingMode) == "" {
		writeDto.TradingMode = storedTradingStrategy.TradingMode
	}

	tradingStrategyDomain, validationError := domains.NewTradingStrategyDomain(writeDto)
	if validationError != nil {
		return entities.TradingStrategy{}, domains.TradingStrategyDomain{}, validationError
	}

	return storedTradingStrategy, tradingStrategyDomain, nil
}

// DeleteTradingStrategy removes the viewer's strategy; the caller checks whether anything still follows it.
func (tradingStrategyService *TradingStrategyService) DeleteTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := tradingStrategyService.findOwnedTradingStrategy(
		executionContext, viewerID, id); findError != nil {
		return findError
	}

	return tradingStrategyService.tradingStrategyRepository.Delete(executionContext, id)
}

// findOwnedTradingStrategy answers a stranger exactly as for a missing strategy, so identifiers cannot be probed for existence.
func (tradingStrategyService *TradingStrategyService) findOwnedTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) (entities.TradingStrategy, error) {
	tradingStrategy, findError := tradingStrategyService.tradingStrategyRepository.FindOne(
		executionContext, id)
	if findError != nil {
		return entities.TradingStrategy{}, findError
	}

	if tradingStrategy.OwnerID != viewerID {
		return entities.TradingStrategy{}, domains.TradingStrategyNotFound(id)
	}

	return tradingStrategy, nil
}
