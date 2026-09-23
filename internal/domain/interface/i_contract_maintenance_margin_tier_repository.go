package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_maintenance_margin_tier_repository.go -destination=mocks/mock_i_contract_maintenance_margin_tier_repository.go -package=mocks

// IContractMaintenanceMarginTierRepository stores perpetual contracts' maintenance
// margin ladders.
type IContractMaintenanceMarginTierRepository interface {
	// ReplaceLadders replaces each named contract's whole ladder with the tiers
	// given, all in one go: either every contract gets its new ladder or none does,
	// and no contract is ever left holding old and new tiers mixed. A contract not
	// named keeps the ladder it has.
	ReplaceLadders(
		executionContext context.Context, laddersBySymbol map[string][]entities.ContractMaintenanceMarginTier,
	) error
	// FindBySymbol is one contract's ladder, first tier first. A contract with none
	// held answers with none.
	FindBySymbol(executionContext context.Context, symbol string) ([]entities.ContractMaintenanceMarginTier, error)
}
