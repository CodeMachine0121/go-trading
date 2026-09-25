package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_position_statistic_archive_proxy.go -destination=mocks/mock_i_contract_position_statistic_archive_proxy.go -package=mocks

// IContractPositionStatisticArchiveProxy reads one UTC day's archive file per call, returned raw (ratios, blanks absent) for the domain to interpret.
// A missing file returns found = false with no error; an error means the archive failed or was unreadable.
type IContractPositionStatisticArchiveProxy interface {
	FetchDailyPositionStatistics(
		executionContext context.Context, symbol string, day time.Time,
	) (statistics []vo.ContractPositionStatisticArchiveVo, found bool, fetchError error)
}
