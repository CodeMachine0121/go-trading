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
	// SaveAllIfAbsent returns how many were stored, leaving already-held statistics unchanged.
	SaveAllIfAbsent(
		executionContext context.Context, statistics []entities.ContractPositionStatistic,
	) (int, error)
	FindLatest(
		executionContext context.Context, symbol string,
	) (entities.ContractPositionStatistic, bool, error)
	// CountInRange includes both ends; a history sync uses it to decide whether a day is worth fetching.
	CountInRange(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) (int, error)
	// FindInRange includes both ends, earliest first, at most limit.
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.ContractPositionStatistic, error)
	// FindStatisticTimesInRange includes both ends, earliest first, at most limit, reading only the times so a caller can pick the few it needs.
	FindStatisticTimesInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]time.Time, error)
	// FindAtTimes returns the symbol's statistics stamped at exactly these times, earliest first; times with none are skipped.
	FindAtTimes(
		executionContext context.Context, symbol string, statisticTimes []time.Time,
	) ([]entities.ContractPositionStatistic, error)
}
