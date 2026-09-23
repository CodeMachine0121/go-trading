package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_position_statistic_proxy.go -destination=mocks/mock_i_contract_position_statistic_proxy.go -package=mocks

// IContractPositionStatisticProxy fetches a perpetual contract's five-minute position
// statistics.
//
// One call, one answer: every statistic from startTime to endTime, both included,
// oldest first. That one statistic is assembled from three separate questions — open
// interest, the long-short split across all accounts, and the split across the
// largest ones — and that the venue has to be asked a few hundred at a time, is this
// contract's to hide.
//
// What it does not hide is an incomplete result: a moment one of the two splits did
// not cover comes back with that split absent, because "a statistic without both is
// not stored" is a rule, and rules belong to the domain.
type IContractPositionStatisticProxy interface {
	FetchPositionStatistics(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) ([]vo.ContractPositionStatisticVo, error)
}
