package service

import (
	"context"
	"maps"
	"slices"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// defaultTradingSymbols are the markets the system ships knowing about. They are
// registered when the schema is built, so the console has something to offer before
// the first candle has ever been ingested.
var defaultTradingSymbols = []string{"BTCUSDT", "ETHUSDT"}

// TradingSymbolService is the application layer's only entry point for trading
// symbols. Its public use-case methods never call one another.
type TradingSymbolService struct {
	tradingSymbolRepository domaininterface.ITradingSymbolRepository
	kCandleRepository       domaininterface.IKCandleRepository
}

func NewTradingSymbolService(
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	kCandleRepository domaininterface.IKCandleRepository,
) *TradingSymbolService {
	return &TradingSymbolService{
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
	}
}

// ListTradingSymbols returns every trading symbol worth asking about: the ones the
// system has been told about, plus the ones it actually holds candles for. Each
// appears once, ordered by name.
//
// Both halves are needed. Registered-but-empty markets are what makes a freshly
// built database usable at all; held-but-unregistered ones are what happens when
// somebody stores a candle for a market nobody registered, and they must stay
// findable afterwards.
func (tradingSymbolService *TradingSymbolService) ListTradingSymbols(
	executionContext context.Context,
) ([]dto.TradingSymbolDto, error) {
	registeredSymbols, findRegisteredError := tradingSymbolService.tradingSymbolRepository.FindAll(
		executionContext)
	if findRegisteredError != nil {
		return nil, findRegisteredError
	}

	heldSymbols, findHeldError := tradingSymbolService.kCandleRepository.FindDistinctSymbols(
		executionContext)
	if findHeldError != nil {
		return nil, findHeldError
	}

	listedSymbols := tradingSymbolService.namesOf(registeredSymbols)
	for _, heldSymbol := range heldSymbols {
		listedSymbols[heldSymbol] = true
	}

	names := slices.Sorted(maps.Keys(listedSymbols))

	tradingSymbolDtos := make([]dto.TradingSymbolDto, 0, len(names))
	for _, name := range names {
		tradingSymbolDtos = append(tradingSymbolDtos, dto.TradingSymbolDto{Symbol: name})
	}

	return tradingSymbolDtos, nil
}

// RegisterDefaultTradingSymbols registers the markets the system ships knowing about
// and reports which of them were new. Already-registered ones are read first and
// left alone, so running this again does nothing and says so.
func (tradingSymbolService *TradingSymbolService) RegisterDefaultTradingSymbols(
	executionContext context.Context,
) ([]string, error) {
	registeredSymbols, findError := tradingSymbolService.tradingSymbolRepository.FindAll(executionContext)
	if findError != nil {
		return nil, findError
	}

	alreadyRegistered := tradingSymbolService.namesOf(registeredSymbols)

	newcomers := make([]entities.TradingSymbol, 0, len(defaultTradingSymbols))
	newcomerNames := make([]string, 0, len(defaultTradingSymbols))
	for _, defaultSymbol := range defaultTradingSymbols {
		if !alreadyRegistered[defaultSymbol] {
			newcomers = append(newcomers, entities.TradingSymbol{Symbol: defaultSymbol})
			newcomerNames = append(newcomerNames, defaultSymbol)
		}
	}

	if registerError := tradingSymbolService.tradingSymbolRepository.RegisterAll(
		executionContext, newcomers); registerError != nil {
		return nil, registerError
	}

	return newcomerNames, nil
}

// namesOf is the set of names those registered symbols carry. Both use cases start
// by asking "is this name already registered?", and answering it twice would give
// the two answers a chance to drift apart.
func (tradingSymbolService *TradingSymbolService) namesOf(
	registeredSymbols []entities.TradingSymbol,
) map[string]bool {
	names := make(map[string]bool, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		names[registeredSymbol.Symbol] = true
	}

	return names
}
