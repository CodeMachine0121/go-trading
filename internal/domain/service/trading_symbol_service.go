package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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
	symbolLookupProxy       domaininterface.ISymbolLookupProxy
	clockProxy              domaininterface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
}

func NewTradingSymbolService(
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	kCandleRepository domaininterface.IKCandleRepository,
	symbolLookupProxy domaininterface.ISymbolLookupProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
) *TradingSymbolService {
	return &TradingSymbolService{
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		symbolLookupProxy:       symbolLookupProxy,
		clockProxy:              clockProxy,
		marketCatalogDomain:     marketCatalogDomain,
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

	registrationsByName := make(map[string]entities.TradingSymbol, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		registrationsByName[registeredSymbol.Symbol] = registeredSymbol
	}
	for _, heldSymbol := range heldSymbols {
		if _, isRegistered := registrationsByName[heldSymbol]; !isRegistered {
			// Holding candles for a market nobody registered is how a symbol created
			// by hand behaves. It stays findable, and reads as belonging to the market
			// this system had before markets were recorded.
			registrationsByName[heldSymbol] = entities.TradingSymbol{Symbol: heldSymbol}
		}
	}

	// Asked for again rather than sifted out of what was already read, because the
	// order the places are handed out in is a rule storage owns. Working it out a
	// second time here would be a second copy of that rule, free to drift.
	watchedSymbols, findWatchedError := tradingSymbolService.tradingSymbolRepository.FindWatched(
		executionContext)
	if findWatchedError != nil {
		return nil, findWatchedError
	}

	currentTime := tradingSymbolService.clockProxy.Now()
	// The same roster the follows are actually handed out from, so what this list
	// promises and what the system does cannot drift apart.
	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedSymbols, tradingSymbolService.marketCatalogDomain, currentTime)

	names := slices.Sorted(maps.Keys(registrationsByName))

	tradingSymbolDtos := make([]dto.TradingSymbolDto, 0, len(names))
	for _, name := range names {
		registration := registrationsByName[name]
		marketDomain := tradingSymbolService.marketCatalogDomain.MarketOf(registration.Market)

		tradingSymbolDtos = append(tradingSymbolDtos, dto.TradingSymbolDto{
			Symbol:                 name,
			Market:                 string(marketDomain.Value()),
			IsWatched:              registration.IsWatched,
			DisplayName:            registration.DisplayName,
			IsWithinTradingSession: marketDomain.IsOpen(currentTime),
			HasTradingSession:      !marketDomain.NeverCloses(),
			HasLiveUpdates:         rosterDomain.HasLiveUpdates(name, marketDomain),
		})
	}

	return tradingSymbolDtos, nil
}

// RegisterDefaultTradingSymbols registers the markets the system ships knowing about
// and reports which of them were new. Already-registered ones are read first and
// left alone, so running this again does nothing and says so.
//
// They are registered as watched. A fresh install whose watchlist was empty would
// fetch nothing at all, and an install upgrading from a watchlist held in settings
// would suddenly stop fetching what it had been fetching all along — so the state
// that keeps both behaving as before is the one to start from.
func (tradingSymbolService *TradingSymbolService) RegisterDefaultTradingSymbols(
	executionContext context.Context,
) ([]string, error) {
	registeredSymbols, findError := tradingSymbolService.tradingSymbolRepository.FindAll(executionContext)
	if findError != nil {
		return nil, findError
	}

	alreadyRegistered := tradingSymbolService.namesOf(registeredSymbols)
	registeredAt := tradingSymbolService.clockProxy.Now().UTC()

	newcomers := make([]entities.TradingSymbol, 0, len(defaultTradingSymbols))
	newcomerNames := make([]string, 0, len(defaultTradingSymbols))
	for _, defaultSymbol := range defaultTradingSymbols {
		if !alreadyRegistered[defaultSymbol] {
			newcomers = append(newcomers, entities.TradingSymbol{
				Symbol:       defaultSymbol,
				Market:       string(vo.MarketCrypto),
				IsWatched:    true,
				RegisteredAt: registeredAt,
			})
			newcomerNames = append(newcomerNames, defaultSymbol)
		}
	}

	if registerError := tradingSymbolService.tradingSymbolRepository.RegisterAll(
		executionContext, newcomers); registerError != nil {
		return nil, registerError
	}

	return newcomerNames, nil
}

// AddToWatchlist starts keeping one market's candles up to date, after checking with
// that market that the code exists.
//
// The check comes first and the write only happens if it passed. Writing first and
// finding out later would turn one typo into a line that fails once every round for
// as long as the system runs, in a log nobody reads — so the round trip is paid here,
// where somebody is still looking at what they typed.
//
// Adding something already watched leaves one entry rather than failing: the caller
// asked for it to be watched, and it is.
func (tradingSymbolService *TradingSymbolService) AddToWatchlist(
	executionContext context.Context, entryDto dto.WatchlistEntryDto,
) error {
	watchlistEntryDomain, validationError := domains.NewWatchlistEntryDomain(
		entryDto, tradingSymbolService.marketCatalogDomain)
	if validationError != nil {
		return validationError
	}

	listing, lookupError := tradingSymbolService.symbolLookupProxy.LookUpSymbol(
		executionContext, watchlistEntryDomain.Market(), watchlistEntryDomain.Symbol())
	if lookupError != nil {
		return fmt.Errorf("%w: %w", domains.ErrMarketDataSourceUnavailable, lookupError)
	}

	if !listing.IsListed {
		return fmt.Errorf("%w: %s 在 %s 找不到這個代號",
			domains.ErrTradingSymbolNotInMarket,
			watchlistEntryDomain.Symbol(), watchlistEntryDomain.Market())
	}

	previouslyRegistered, _, findError := tradingSymbolService.tradingSymbolRepository.FindBySymbol(
		executionContext, watchlistEntryDomain.Symbol())
	if findError != nil {
		return findError
	}

	// The name is written every time, so whatever the venue says now is what the
	// watchlist reads — adding a symbol back is also how a renamed company gets its
	// new name, and nobody has to know that.
	return tradingSymbolService.tradingSymbolRepository.Save(
		executionContext,
		watchlistEntryDomain.ToEntity(
			listing, previouslyRegistered.RegisteredAt, tradingSymbolService.clockProxy.Now()))
}

// RemoveFromWatchlist stops keeping one market's candles up to date.
//
// It removes nothing else. The system still knows the market, and every candle it
// ever fetched is still there to query, chart and replay — because not wanting to
// watch something any more and not wanting its history are different wishes, and
// granting the second one when only the first was asked for is not recoverable.
//
// Removing something that was not being watched is not a failure: what was asked for
// is already true.
func (tradingSymbolService *TradingSymbolService) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	tradingSymbolDomain, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return fmt.Errorf("%w: %w", domains.ErrWatchlistEntryValidation, symbolError)
	}

	registeredSymbol, isRegistered, findError := tradingSymbolService.tradingSymbolRepository.
		FindBySymbol(executionContext, tradingSymbolDomain.Value())
	if findError != nil {
		return findError
	}

	if !isRegistered || !registeredSymbol.IsWatched {
		return nil
	}

	registeredSymbol.IsWatched = false

	return tradingSymbolService.tradingSymbolRepository.Save(executionContext, registeredSymbol)
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
