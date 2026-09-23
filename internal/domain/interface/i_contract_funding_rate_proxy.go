package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_funding_rate_proxy.go -destination=mocks/mock_i_contract_funding_rate_proxy.go -package=mocks

// IContractFundingRateProxy fetches a perpetual contract's funding rate settlements.
//
// One call, one answer: every settlement strictly after the given moment, up to now,
// oldest first. A zero moment means from the very first settlement the contract ever
// had. That the venue answers a page at a time, and how the first page has to be
// asked for, is this contract's to hide.
//
// Nothing is judged here. A settlement the venue recorded no mark price for comes
// back with that figure absent, because what an absent mark price means is a rule,
// and rules belong to the domain.
type IContractFundingRateProxy interface {
	FetchFundingRateSettlements(
		executionContext context.Context, symbol string, after time.Time,
	) ([]vo.ContractFundingRateSettlementVo, error)
}
