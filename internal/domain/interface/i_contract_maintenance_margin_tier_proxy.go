package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_maintenance_margin_tier_proxy.go -destination=mocks/mock_i_contract_maintenance_margin_tier_proxy.go -package=mocks

// IContractMaintenanceMarginTierProxy fetches every contract's ladder using account credentials: ErrContractAccountCredentialsMissing if none are configured, ErrContractAccountCredentialsRefused if the venue rejects them.
type IContractMaintenanceMarginTierProxy interface {
	FetchMaintenanceMarginLadders(executionContext context.Context) ([]vo.ContractMaintenanceMarginLadderVo, error)
}
