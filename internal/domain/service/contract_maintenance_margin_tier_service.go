package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// ContractMaintenanceMarginTierService keeps every contract's maintenance margin ladder current; without an account the refresh returns ErrContractAccountCredentialsMissing, which is not a failure.
type ContractMaintenanceMarginTierService struct {
	tierRepository                  domaininterface.IContractMaintenanceMarginTierRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	tierProxy                       domaininterface.IContractMaintenanceMarginTierProxy
	clockProxy                      domaininterface.IClockProxy
}

func NewContractMaintenanceMarginTierService(
	tierRepository domaininterface.IContractMaintenanceMarginTierRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	tierProxy domaininterface.IContractMaintenanceMarginTierProxy,
	clockProxy domaininterface.IClockProxy,
) *ContractMaintenanceMarginTierService {
	return &ContractMaintenanceMarginTierService{
		tierRepository:                  tierRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		tierProxy:                       tierProxy,
		clockProxy:                      clockProxy,
	}
}

// RefreshLadders replaces each contract's ladder when the venue reports a valid one; unreported or invalid ladders are kept (invalid ones are named in the report), and a venue failure changes nothing.
func (tierService *ContractMaintenanceMarginTierService) RefreshLadders(
	executionContext context.Context,
) (dto.ContractMaintenanceMarginRefreshReportDto, error) {
	reportedLadders, fetchError := tierService.tierProxy.FetchMaintenanceMarginLadders(executionContext)
	if errors.Is(fetchError, domains.ErrContractAccountCredentialsMissing) {
		return dto.ContractMaintenanceMarginRefreshReportDto{}, fetchError
	}
	if fetchError != nil {
		return dto.ContractMaintenanceMarginRefreshReportDto{}, fmt.Errorf(
			"%w: %w", domains.ErrMarketDataSourceUnavailable, fetchError)
	}

	registeredSymbols, findError := tierService.contractTradingSymbolRepository.FindAll(executionContext)
	if findError != nil {
		return dto.ContractMaintenanceMarginRefreshReportDto{}, findError
	}
	isRegistered := make(map[string]bool, len(registeredSymbols))
	for _, registeredSymbol := range registeredSymbols {
		isRegistered[registeredSymbol.Symbol] = true
	}

	confirmedAt := tierService.clockProxy.Now()
	laddersBySymbol := make(map[string][]entities.ContractMaintenanceMarginTier)
	refusedLadders := make([]dto.RefusedMaintenanceLadderDto, 0)
	for _, reportedLadder := range reportedLadders {
		if !isRegistered[reportedLadder.Symbol] {
			continue
		}

		ladderDomain, validationError := domains.NewContractMaintenanceMarginLadderDomain(reportedLadder)
		if validationError != nil {
			refusedLadders = append(refusedLadders, dto.RefusedMaintenanceLadderDto{
				Symbol: reportedLadder.Symbol, Reason: validationError.Error(),
			})
			continue
		}
		laddersBySymbol[ladderDomain.Symbol()] = ladderDomain.ToEntities(confirmedAt)
	}

	if len(laddersBySymbol) > 0 {
		if replaceError := tierService.tierRepository.ReplaceLadders(
			executionContext, laddersBySymbol); replaceError != nil {
			return dto.ContractMaintenanceMarginRefreshReportDto{}, replaceError
		}
	}

	return dto.ContractMaintenanceMarginRefreshReportDto{
		RefreshedCount: len(laddersBySymbol),
		RefusedLadders: refusedLadders,
	}, nil
}

// FindTiers returns one contract's ladder, first tier first; none held is not an error.
func (tierService *ContractMaintenanceMarginTierService) FindTiers(
	executionContext context.Context, symbol string,
) ([]dto.ContractMaintenanceMarginTierDto, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return nil, fmt.Errorf("%w: %w", domains.ErrContractMaintenanceMarginTierValidation, symbolError)
	}

	tiers, findError := tierService.tierRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return nil, findError
	}

	tierDtos := make([]dto.ContractMaintenanceMarginTierDto, 0, len(tiers))
	for _, tier := range tiers {
		tierDtos = append(tierDtos, tier.ToDto())
	}

	return tierDtos, nil
}
