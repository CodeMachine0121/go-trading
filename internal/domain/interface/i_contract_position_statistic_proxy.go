package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_position_statistic_proxy.go -destination=mocks/mock_i_contract_position_statistic_proxy.go -package=mocks

// IContractPositionStatisticProxy returns five-minute statistics from startTime to endTime inclusive, oldest first, merged from three venue endpoints.
// A moment missing one of the long-short splits comes back with it absent; whether to store it is the domain's rule.
type IContractPositionStatisticProxy interface {
	FetchPositionStatistics(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) ([]vo.ContractPositionStatisticVo, error)
}
