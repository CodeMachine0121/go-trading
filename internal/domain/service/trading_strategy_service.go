package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// TradingStrategyService is the application layer's only entry point for trading
// strategies. Its public use-case methods never call one another.
//
// It is given no way to reach a strategy script and no way to reach a bot, and that
// is the point. Whether a source may name a strategy script is the strategy script
// rules' question; whether anything is currently following these rules is the bot's;
// and joining those to this is the application layer's job. A dependency that is not
// here cannot be reached for by accident later.
type TradingStrategyService struct {
	tradingStrategyRepository domaininterface.ITradingStrategyRepository
}

func NewTradingStrategyService(
	tradingStrategyRepository domaininterface.ITradingStrategyRepository,
) *TradingStrategyService {
	return &TradingStrategyService{tradingStrategyRepository: tradingStrategyRepository}
}

// CreateTradingStrategy saves a new set of rules for its owner and hands it back as
// stored. One that breaks a rule is refused before anything is written.
func (tradingStrategyService *TradingStrategyService) CreateTradingStrategy(
	executionContext context.Context, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	// An identifier arriving on a create would rewrite whichever trading strategy
	// it named, including somebody else's. Clearing it makes creating unable to
	// mean that.
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

// ListTradingStrategies returns this person's trading strategies, by name.
//
// Having none is an empty list rather than a refusal: it is the ordinary state of
// somebody who has not built one yet.
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

// GetTradingStrategy returns the viewer's own trading strategy carrying this
// identifier, sources and both trees included.
//
// This is also the read a round uses. A round has no signed-in caller — the clock
// started it — so it asks as the person who owns the bot, and gets exactly the same
// answer under exactly the same rule. One read rather than two is what keeps "what a
// round runs" and "what its owner sees" from ever drifting apart.
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

// UpdateTradingStrategy rewrites the viewer's own trading strategy.
//
// Whether any bot following it is running is asked by the caller, because a bot is
// not this service's to read. Every rule that applied to creating it applies here
// word for word, because both arrive as the same shape and are checked by the same
// model.
func (tradingStrategyService *TradingStrategyService) UpdateTradingStrategy(
	executionContext context.Context, viewerID uint, writeDto dto.TradingStrategyWriteDto,
) (dto.TradingStrategyDto, error) {
	storedTradingStrategy, findError := tradingStrategyService.findOwnedTradingStrategy(
		executionContext, viewerID, writeDto.ID)
	if findError != nil {
		return dto.TradingStrategyDto{}, findError
	}

	// The owner comes from what is stored, never from what arrived. A trading
	// strategy cannot change hands, and the write path not being able to say so is
	// stronger than remembering not to.
	writeDto.OwnerID = storedTradingStrategy.OwnerID

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

// DeleteTradingStrategy removes the viewer's own trading strategy.
//
// Whether anything still follows it is asked by the caller, for the reason a rewrite
// is: a bot is not this service's to read.
func (tradingStrategyService *TradingStrategyService) DeleteTradingStrategy(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := tradingStrategyService.findOwnedTradingStrategy(
		executionContext, viewerID, id); findError != nil {
		return findError
	}

	return tradingStrategyService.tradingStrategyRepository.Delete(executionContext, id)
}

// findOwnedTradingStrategy is the one place "is this theirs" is answered, so that
// somebody else's and a missing one give the same refusal in every use case. Told
// apart anywhere, anybody could walk the identifiers and learn what exists.
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
