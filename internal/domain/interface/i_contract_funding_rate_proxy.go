package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_funding_rate_proxy.go -destination=mocks/mock_i_contract_funding_rate_proxy.go -package=mocks

// IContractFundingRateProxy returns every settlement strictly after the given moment (zero means from the first ever), oldest first, hiding venue paging.
// A settlement with no recorded mark price comes back with it absent; judging that is the domain's job.
type IContractFundingRateProxy interface {
	FetchFundingRateSettlements(
		executionContext context.Context, symbol string, after time.Time,
	) ([]vo.ContractFundingRateSettlementVo, error)
}
