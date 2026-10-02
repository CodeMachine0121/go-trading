package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_contract_history_sync_run_repository.go -destination=mocks/mock_i_k_candle_contract_history_sync_run_repository.go -package=mocks

// IKCandleContractHistorySyncRunRepository persists runs before the first fetch, since the row is the only record of work in progress.
type IKCandleContractHistorySyncRunRepository interface {
	Save(
		executionContext context.Context, syncRun entities.KCandleContractHistorySyncRun,
	) (entities.KCandleContractHistorySyncRun, error)
	FindOne(
		executionContext context.Context, id uint,
	) (entities.KCandleContractHistorySyncRun, bool, error)
	CountRunning(executionContext context.Context) (int, error)
	// FailRunningOutside closes running runs whose replica is not among liveReplicaNames, since that replica is gone and so is the fetching.
	FailRunningOutside(
		executionContext context.Context, liveReplicaNames []string, reason string, finishedAt time.Time,
	) (int, error)
}
