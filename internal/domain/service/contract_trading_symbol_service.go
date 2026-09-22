package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// ContractTradingSymbolService is the application layer's only entry point for the
// perpetual contracts the system knows about. Its public use-case methods never call
// one another.
//
// It shares nothing with the spot list, which is the point: the same code names a
// different instrument on each venue, so following BTCUSDT here neither implies nor
// disturbs following BTCUSDT there.
type ContractTradingSymbolService struct {
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	kCandleContractRepository       domaininterface.IKCandleContractRepository
	contractSymbolLookupProxy       domaininterface.IContractSymbolLookupProxy
}

func NewContractTradingSymbolService(
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	contractSymbolLookupProxy domaininterface.IContractSymbolLookupProxy,
) *ContractTradingSymbolService {
	return &ContractTradingSymbolService{
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		kCandleContractRepository:       kCandleContractRepository,
		contractSymbolLookupProxy:       contractSymbolLookupProxy,
	}
}

// ListContractTradingSymbols returns every perpetual contract worth asking about: the
// ones the system has been told about, plus the ones it actually holds candles for.
// Each appears once, ordered by name.
//
// Both halves are needed, for the same reasons the spot list needs them:
// registered-but-empty contracts are what makes a freshly built database usable at
// all, and held-but-unregistered ones are what a hand-placed candle leaves behind.
func (contractTradingSymbolService *ContractTradingSymbolService) ListContractTradingSymbols(
	executionContext context.Context,
) ([]dto.ContractTradingSymbolDto, error) {
	registeredSymbols, findRegisteredError := contractTradingSymbolService.
		contractTradingSymbolRepository.FindAll(executionContext)
	if findRegisteredError != nil {
		return nil, findRegisteredError
	}

	symbolsHoldingCandles, findHeldError := contractTradingSymbolService.
		kCandleContractRepository.FindDistinctSymbols(executionContext)
	if findHeldError != nil {
		return nil, findHeldError
	}

	// A registered contract speaks for itself; one that is only known because a candle
	// was stored for it has nobody to follow it, so it is added as unwatched and
	// cannot displace a registration that says otherwise.
	dtoBySymbol := make(map[string]dto.ContractTradingSymbolDto, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		dtoBySymbol[registeredSymbol.Symbol] = registeredSymbol.ToDto()
	}
	for _, heldSymbol := range symbolsHoldingCandles {
		if _, alreadyKnown := dtoBySymbol[heldSymbol]; !alreadyKnown {
			dtoBySymbol[heldSymbol] = dto.ContractTradingSymbolDto{Symbol: heldSymbol}
		}
	}

	symbolNames := make([]string, 0, len(dtoBySymbol))
	for symbolName := range dtoBySymbol {
		symbolNames = append(symbolNames, symbolName)
	}
	slices.Sort(symbolNames)

	contractTradingSymbolDtos := make([]dto.ContractTradingSymbolDto, 0, len(symbolNames))
	for _, symbolName := range symbolNames {
		contractTradingSymbolDtos = append(contractTradingSymbolDtos, dtoBySymbol[symbolName])
	}

	return contractTradingSymbolDtos, nil
}

// AddToWatchlist starts keeping one perpetual contract's candles up to date, after
// checking with the contract venue that the code exists.
//
// The check comes first and the write only happens if it passed. Writing first and
// finding out later would turn one typo into a line that fails once every round for
// as long as the system runs, in a log nobody reads.
//
// A venue that cannot be reached at all leaves the watchlist untouched, which is the
// opposite of what a failed catch-up does later: here nothing has been established
// yet, so there is nothing to keep.
//
// Adding something already watched leaves one entry rather than failing: the caller
// asked for it to be watched, and it is.
func (contractTradingSymbolService *ContractTradingSymbolService) AddToWatchlist(
	executionContext context.Context, symbol string,
) error {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return fmt.Errorf("%w: %w", domains.ErrWatchlistEntryValidation, symbolError)
	}

	listing, lookupError := contractTradingSymbolService.contractSymbolLookupProxy.LookUpSymbol(
		executionContext, contractSymbol.Value())
	if lookupError != nil {
		return fmt.Errorf("%w: %w", domains.ErrMarketDataSourceUnavailable, lookupError)
	}

	if !listing.IsListed {
		return fmt.Errorf("%w: 永續合約找不到 %s 這個代號",
			domains.ErrTradingSymbolNotInMarket, contractSymbol.Value())
	}

	return contractTradingSymbolService.contractTradingSymbolRepository.Save(
		executionContext,
		entities.ContractTradingSymbol{Symbol: contractSymbol.Value(), IsWatched: true},
	)
}

// RemoveFromWatchlist stops keeping one perpetual contract's candles up to date.
//
// It only stops the following. The contract stays registered and every candle already
// fetched stays exactly where it is, and the spot list of the same name is not touched
// at all.
//
// Removing something that was not being followed is not a failure: what was asked for
// is already true.
func (contractTradingSymbolService *ContractTradingSymbolService) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return fmt.Errorf("%w: %w", domains.ErrWatchlistEntryValidation, symbolError)
	}

	registeredSymbol, isRegistered, findError := contractTradingSymbolService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return findError
	}

	if !isRegistered || !registeredSymbol.IsWatched {
		return nil
	}

	registeredSymbol.IsWatched = false

	return contractTradingSymbolService.contractTradingSymbolRepository.Save(
		executionContext, registeredSymbol)
}
