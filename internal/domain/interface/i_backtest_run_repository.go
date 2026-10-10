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
	// LockOldestQueued reads the oldest queued run and locks it until the surrounding transaction ends, skipping runs another replica holds, so two replicas never claim the same run.
	LockOldestQueued(executionContext context.Context) (entities.BacktestRun, bool, error)
	// SaveClaim writes the claimed run's status, replica and start.
	SaveClaim(executionContext context.Context, backtestRun entities.BacktestRun) error
	// UpdateIfRunning writes the run's progress or ending only while it is still running under the same replica, and says whether it was.
	UpdateIfRunning(executionContext context.Context, backtestRun entities.BacktestRun) (bool, error)
	// CancelIfActive ends a queued or running run and says whether it was still active.
	CancelIfActive(executionContext context.Context, backtestRun entities.BacktestRun) (bool, error)
	FailRunningOutside(
		executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
	) (int, error)
	DeleteFinishedBefore(executionContext context.Context, cutoff time.Time) (int, error)
}
