package service

import (
	"sort"

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
func (tradingSymbolService *TradingSymbolService) ListTradingSymbols() ([]dto.TradingSymbolDto, error) {
	registeredSymbols, findRegisteredError := tradingSymbolService.tradingSymbolRepository.FindAll()
	if findRegisteredError != nil {
		return nil, findRegisteredError
	}

	heldSymbols, findHeldError := tradingSymbolService.kCandleRepository.FindDistinctSymbols()
	if findHeldError != nil {
		return nil, findHeldError
	}

	listedSymbols := make(map[string]bool, len(registeredSymbols)+len(heldSymbols))
	for _, registeredSymbol := range registeredSymbols {
		listedSymbols[registeredSymbol.Symbol] = true
	}
	for _, heldSymbol := range heldSymbols {
		listedSymbols[heldSymbol] = true
	}

	names := make([]string, 0, len(listedSymbols))
	for name := range listedSymbols {
		names = append(names, name)
	}
	sort.Strings(names)

	tradingSymbolDtos := make([]dto.TradingSymbolDto, 0, len(names))
	for _, name := range names {
		tradingSymbolDtos = append(tradingSymbolDtos, dto.TradingSymbolDto{Symbol: name})
	}

	return tradingSymbolDtos, nil
}

// RegisterDefaultTradingSymbols registers the markets the system ships knowing about
// and reports which of them were new. Already-registered ones are read first and
// left alone, so running this again does nothing and says so.
func (tradingSymbolService *TradingSymbolService) RegisterDefaultTradingSymbols() ([]string, error) {
	registeredSymbols, findError := tradingSymbolService.tradingSymbolRepository.FindAll()
	if findError != nil {
		return nil, findError
	}

	alreadyRegistered := make(map[string]bool, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		alreadyRegistered[registeredSymbol.Symbol] = true
	}

	newcomers := make([]entities.TradingSymbol, 0, len(defaultTradingSymbols))
	newcomerNames := make([]string, 0, len(defaultTradingSymbols))
	for _, defaultSymbol := range defaultTradingSymbols {
		if !alreadyRegistered[defaultSymbol] {
			newcomers = append(newcomers, entities.TradingSymbol{Symbol: defaultSymbol})
			newcomerNames = append(newcomerNames, defaultSymbol)
		}
	}

	if registerError := tradingSymbolService.tradingSymbolRepository.RegisterAll(newcomers); registerError != nil {
		return nil, registerError
	}

	return newcomerNames, nil
}
