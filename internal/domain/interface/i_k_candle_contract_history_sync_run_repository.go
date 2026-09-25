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
	// FailAllRunning closes runs a restart cut short.
	FailAllRunning(
		executionContext context.Context, reason string, finishedAt time.Time,
	) (int, error)
}
