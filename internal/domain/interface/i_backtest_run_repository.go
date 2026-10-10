package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_backtest_run_repository.go -destination=mocks/mock_i_backtest_run_repository.go -package=mocks

// IBacktestRunRepository keeps the runs replicas claim and callers poll; the conditional writes guard against a run cancelled or swept while its replica still writes to it.
type IBacktestRunRepository interface {
	Save(executionContext context.Context, backtestRun entities.BacktestRun) (entities.BacktestRun, error)
	FindOneOwned(executionContext context.Context, ownerID uint, id uint) (entities.BacktestRun, bool, error)
	FindPageByOwner(
		executionContext context.Context, ownerID uint, query dto.BacktestRunListQueryDto,
	) ([]entities.BacktestRun, int64, error)
	// CountActiveByOwner counts queued and running runs, the only ones the owner limit is about.
	CountActiveByOwner(executionContext context.Context, ownerID uint) (int, error)
	// ClaimOldestQueued moves the oldest queued run to running under this replica's name, or reports there was none; two replicas never get the same run.
	ClaimOldestQueued(
		executionContext context.Context, replicaName string, startedAt time.Time,
	) (entities.BacktestRun, bool, error)
	// UpdateIfRunning writes the run's progress or ending only while it is still running under the same replica, and says whether it was.
	UpdateIfRunning(executionContext context.Context, backtestRun entities.BacktestRun) (bool, error)
	// CancelIfActive ends a queued or running run and says whether it was still active.
	CancelIfActive(executionContext context.Context, backtestRun entities.BacktestRun) (bool, error)
	FailRunningOutside(
		executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
	) (int, error)
	DeleteFinishedBefore(executionContext context.Context, cutoff time.Time) (int, error)
}
