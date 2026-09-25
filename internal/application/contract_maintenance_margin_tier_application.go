package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type ContractMaintenanceMarginTierApplication struct {
	tierService *service.ContractMaintenanceMarginTierService
}

func NewContractMaintenanceMarginTierApplication(
	tierService *service.ContractMaintenanceMarginTierService,
) *ContractMaintenanceMarginTierApplication {
	return &ContractMaintenanceMarginTierApplication{tierService: tierService}
}

func (tierApplication *ContractMaintenanceMarginTierApplication) RefreshLadders(
	executionContext context.Context,
) (dto.ContractMaintenanceMarginRefreshReportDto, error) {
	return tierApplication.tierService.RefreshLadders(executionContext)
}

// GetTiers returns one contract's ladder, first tier first.
func (tierApplication *ContractMaintenanceMarginTierApplication) GetTiers(
	executionContext context.Context, symbol string,
) ([]dto.ContractMaintenanceMarginTierDto, error) {
	return tierApplication.tierService.FindTiers(executionContext, symbol)
}
