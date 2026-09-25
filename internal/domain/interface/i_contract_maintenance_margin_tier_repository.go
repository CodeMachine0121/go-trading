package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_maintenance_margin_tier_repository.go -destination=mocks/mock_i_contract_maintenance_margin_tier_repository.go -package=mocks

type IContractMaintenanceMarginTierRepository interface {
	// ReplaceLadders atomically replaces each named contract's whole ladder; contracts not named keep theirs.
	ReplaceLadders(
		executionContext context.Context, laddersBySymbol map[string][]entities.ContractMaintenanceMarginTier,
	) error
	// FindBySymbol returns the ladder first tier first, empty when none is held.
	FindBySymbol(executionContext context.Context, symbol string) ([]entities.ContractMaintenanceMarginTier, error)
}
