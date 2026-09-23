package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_maintenance_margin_tier_proxy.go -destination=mocks/mock_i_contract_maintenance_margin_tier_proxy.go -package=mocks

// IContractMaintenanceMarginTierProxy fetches every perpetual contract's maintenance
// margin ladder in one answer.
//
// The venue only tells an account this, so a question has to prove whose it is. With
// no account configured, the answer is ErrContractAccountCredentialsMissing and
// nothing is asked; the venue refusing the account's key is
// ErrContractAccountCredentialsRefused. How the proof is made is this contract's to
// hide.
type IContractMaintenanceMarginTierProxy interface {
	FetchMaintenanceMarginLadders(executionContext context.Context) ([]vo.ContractMaintenanceMarginLadderVo, error)
}
