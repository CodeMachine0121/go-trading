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

// ContractMaintenanceMarginTierService keeps every known perpetual contract's full
// maintenance margin ladder current, and answers questions about it. Its public use
// cases never call one another.
//
// The ladder is something only an account is told. When no account is configured the
// refresh says so with ErrContractAccountCredentialsMissing, which is not a failure:
// it is how the rest of the system knows to go without.
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

// RefreshLadders replaces the ladder of every registered contract the venue reports a
// sensible one for, and says what it did.
//
// A contract the venue did not report keeps its ladder. A contract whose reported
// ladder cannot be one keeps its ladder too, and is named in the report — only that
// contract: one broken ladder is no reason to keep every other one stale. The venue
// failing, or refusing the account, changes nothing at all.
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

// FindTiers is one contract's ladder, first tier first. A contract with none held
// answers with none, which is not an error.
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
