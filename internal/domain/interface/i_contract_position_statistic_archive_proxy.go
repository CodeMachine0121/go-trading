package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_position_statistic_archive_proxy.go -destination=mocks/mock_i_contract_position_statistic_archive_proxy.go -package=mocks

// IContractPositionStatisticArchiveProxy reads a perpetual contract's position
// statistics out of the venue's history archive, one day at a time.
//
// **A day is the only question it answers**, because that is how the archive is
// kept: one file per contract per calendar day in UTC. Every statistic in it comes
// back in the order the file keeps them, in the archive's own shape — ratios rather
// than shares, and any figure the file left blank absent — because working the
// shares out, and deciding what a blank means, are rules, and rules belong to the
// domain.
//
// **A day with no file is an answer, not a failure**: the day has not been published
// yet, or the contract did not exist, and either way there is nothing to store. It
// comes back as found = false with no error. An error is the archive not answering,
// or answering with something that cannot be read.
type IContractPositionStatisticArchiveProxy interface {
	FetchDailyPositionStatistics(
		executionContext context.Context, symbol string, day time.Time,
	) (statistics []vo.ContractPositionStatisticArchiveVo, found bool, fetchError error)
}
