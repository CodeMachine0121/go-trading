package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractMaintenanceMarginTierApplication is every use case about perpetual
// contracts' maintenance margin ladders: refreshing them, and reading one back.
type ContractMaintenanceMarginTierApplication struct {
	tierService *service.ContractMaintenanceMarginTierService
}

func NewContractMaintenanceMarginTierApplication(
	tierService *service.ContractMaintenanceMarginTierService,
) *ContractMaintenanceMarginTierApplication {
	return &ContractMaintenanceMarginTierApplication{tierService: tierService}
}

// RefreshLadders refreshes every known contract's ladder.
func (tierApplication *ContractMaintenanceMarginTierApplication) RefreshLadders(
	executionContext context.Context,
) (dto.ContractMaintenanceMarginRefreshReportDto, error) {
	return tierApplication.tierService.RefreshLadders(executionContext)
}

// GetTiers reads one contract's ladder back, first tier first.
func (tierApplication *ContractMaintenanceMarginTierApplication) GetTiers(
	executionContext context.Context, symbol string,
) ([]dto.ContractMaintenanceMarginTierDto, error) {
	return tierApplication.tierService.FindTiers(executionContext, symbol)
}
