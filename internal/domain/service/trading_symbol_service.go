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

// defaultTradingSymbols are registered at schema build so the console has something to offer before any ingestion.
var defaultTradingSymbols = []string{"BTCUSDT", "ETHUSDT"}

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

// ListTradingSymbols returns registered symbols plus unregistered ones that have stored candles, once each and sorted by name, so hand-stored symbols stay findable.
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
			// A hand-created symbol reads as belonging to the market used before markets were recorded.
			registrationsByName[heldSymbol] = entities.TradingSymbol{Symbol: heldSymbol}
		}
	}

	// Re-read rather than filtered locally, because the order places are handed out in is a rule storage owns.
	watchedSymbols, findWatchedError := tradingSymbolService.tradingSymbolRepository.FindWatched(
		executionContext)
	if findWatchedError != nil {
		return nil, findWatchedError
	}

	currentTime := tradingSymbolService.clockProxy.Now()
	// Same roster the follows are handed out from, so this list cannot drift from what the system does.
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

// RegisterDefaultTradingSymbols registers any missing default symbols as watched and returns the new ones, so reruns do nothing.
// They start watched so fresh installs and upgrades from settings-based watchlists keep fetching them.
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

// AddToWatchlist verifies the code with the market before saving, so a typo is caught now instead of failing every round; re-adding is idempotent.
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

	// The display name is rewritten every time, so re-adding picks up a company rename.
	return tradingSymbolService.tradingSymbolRepository.Save(
		executionContext,
		watchlistEntryDomain.ToEntity(
			listing, previouslyRegistered.RegisteredAt, tradingSymbolService.clockProxy.Now()))
}

// RemoveFromWatchlist only stops fetching; the registration and all stored candles are kept, and removing an unwatched symbol is not a failure.
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

// namesOf is the set of registered names.
func (tradingSymbolService *TradingSymbolService) namesOf(
	registeredSymbols []entities.TradingSymbol,
) map[string]bool {
	names := make(map[string]bool, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		names[registeredSymbol.Symbol] = true
	}

	return names
}
