package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_history_sync_run_repository.go -destination=mocks/mock_i_k_candle_history_sync_run_repository.go -package=mocks

// IKCandleHistorySyncRunRepository persists runs before fetching and updates them as they go, since syncs outlive the request that started them.
type IKCandleHistorySyncRunRepository interface {
	Save(
		executionContext context.Context, syncRun entities.KCandleHistorySyncRun,
	) (entities.KCandleHistorySyncRun, error)
	FindOne(
		executionContext context.Context, id uint,
	) (entities.KCandleHistorySyncRun, bool, error)
	// CountRunning is the source of truth for the concurrency limit, since runs live only as rows.
	CountRunning(executionContext context.Context) (int, error)
	// FailRunningOutside marks running runs failed whose replica is not among liveReplicaNames, since that replica is gone and so is the fetching.
	FailRunningOutside(
		executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
	) (int, error)
}
