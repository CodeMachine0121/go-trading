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

// ContractTradingSymbolService is the application layer's only entry point for perpetual contracts; it is independent of the spot list, as the same code names a different instrument per venue.
type ContractTradingSymbolService struct {
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	kCandleContractRepository       domaininterface.IKCandleContractRepository
	contractSymbolLookupProxy       domaininterface.IContractSymbolLookupProxy
	clockProxy                      domaininterface.IClockProxy
}

func NewContractTradingSymbolService(
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	contractSymbolLookupProxy domaininterface.IContractSymbolLookupProxy,
	clockProxy domaininterface.IClockProxy,
) *ContractTradingSymbolService {
	return &ContractTradingSymbolService{
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		kCandleContractRepository:       kCandleContractRepository,
		contractSymbolLookupProxy:       contractSymbolLookupProxy,
		clockProxy:                      clockProxy,
	}
}

// ListContractTradingSymbols returns registered contracts plus any holding candles without a registration, once each, ordered by name.
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

	// Candle-only contracts are added as unwatched and never displace a registration.
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

// AddToWatchlist verifies the contract with the venue before writing, so a typo never becomes a permanently failing entry; an unreachable venue changes nothing, and re-adding is idempotent.
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

	watchedSymbol := entities.ContractTradingSymbol{Symbol: contractSymbol.Value(), IsWatched: true}

	// Record the specification from the same answer; an unparsable one does not block following and is fixed by the daily refresh.
	specificationDomain, specificationError := domains.NewContractTradingSpecificationDomain(
		listing.Specification)
	if specificationError == nil {
		watchedSymbol = specificationDomain.ApplyTo(watchedSymbol, contractTradingSymbolService.clockProxy.Now())
	}

	return contractTradingSymbolService.contractTradingSymbolRepository.Save(executionContext, watchedSymbol)
}

// RefreshTradingSpecifications updates every registered contract (watched or not) and returns how many changed; unlisted contracts keep their last confirmed specification.
func (contractTradingSymbolService *ContractTradingSymbolService) RefreshTradingSpecifications(
	executionContext context.Context,
) (int, error) {
	registeredSymbols, findError := contractTradingSymbolService.contractTradingSymbolRepository.
		FindAll(executionContext)
	if findError != nil {
		return 0, findError
	}

	reportedSpecifications, fetchError := contractTradingSymbolService.contractSymbolLookupProxy.
		FetchTradingSpecifications(executionContext)
	if fetchError != nil {
		return 0, fmt.Errorf("%w: %w", domains.ErrMarketDataSourceUnavailable, fetchError)
	}

	specificationsBySymbol := make(map[string]domains.ContractTradingSpecificationDomain, len(reportedSpecifications))
	for _, reportedSpecification := range reportedSpecifications {
		specificationDomain, specificationError := domains.NewContractTradingSpecificationDomain(
			reportedSpecification)
		if specificationError != nil {
			continue
		}
		specificationsBySymbol[reportedSpecification.Symbol] = specificationDomain
	}

	confirmedAt := contractTradingSymbolService.clockProxy.Now()
	refreshedSymbols := make([]entities.ContractTradingSymbol, 0, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		specificationDomain, isListed := specificationsBySymbol[registeredSymbol.Symbol]
		if !isListed {
			continue
		}
		refreshedSymbols = append(refreshedSymbols, specificationDomain.ApplyTo(registeredSymbol, confirmedAt))
	}

	if len(refreshedSymbols) == 0 {
		return 0, nil
	}

	if saveError := contractTradingSymbolService.contractTradingSymbolRepository.SaveTradingSpecifications(
		executionContext, refreshedSymbols); saveError != nil {
		return 0, saveError
	}

	return len(refreshedSymbols), nil
}

// RemoveFromWatchlist stops following a contract but keeps its registration and candles; removing an unwatched one is not an error.
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
