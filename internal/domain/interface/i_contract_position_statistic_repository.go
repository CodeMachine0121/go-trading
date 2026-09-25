package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_position_statistic_repository.go -destination=mocks/mock_i_contract_position_statistic_repository.go -package=mocks

// IContractPositionStatisticRepository stores perpetual contract position statistics.
type IContractPositionStatisticRepository interface {
	// SaveAllIfAbsent stores every statistic nothing is held for yet and says how
	// many it stored. One already held is left exactly as it was.
	SaveAllIfAbsent(
		executionContext context.Context, statistics []entities.ContractPositionStatistic,
	) (int, error)
	// FindLatest is the most recent statistic held for the contract, and whether
	// there is one at all.
	FindLatest(
		executionContext context.Context, symbol string,
	) (entities.ContractPositionStatistic, bool, error)
	// CountInRange is how many statistics are held for the contract whose statistic
	// time falls between the two, both ends included. A history sync reads it only to
	// decide whether a day is worth asking the archive about.
	CountInRange(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) (int, error)
	// FindInRange returns the statistics whose statistic time falls inside the
	// query's range, both ends included, earliest first, at most limit of them.
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.ContractPositionStatistic, error)
}
