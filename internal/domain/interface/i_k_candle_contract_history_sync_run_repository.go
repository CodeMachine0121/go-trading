package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_contract_history_sync_run_repository.go -destination=mocks/mock_i_k_candle_contract_history_sync_run_repository.go -package=mocks

// IKCandleContractHistorySyncRunRepository stores and retrieves contract history
// sync runs. The row is written before the first candle is fetched, because the run
// exists nowhere else — that row is the work in progress.
type IKCandleContractHistorySyncRunRepository interface {
	Save(
		executionContext context.Context, syncRun entities.KCandleContractHistorySyncRun,
	) (entities.KCandleContractHistorySyncRun, error)
	FindOne(
		executionContext context.Context, id uint,
	) (entities.KCandleContractHistorySyncRun, bool, error)
	// FailAllRunning closes off runs a restart cut short. Work that only lived in the
	// stopped process left with it, so a row still claiming to be running is claiming
	// something nobody is doing.
	FailAllRunning(
		executionContext context.Context, reason string, finishedAt time.Time,
	) (int, error)
}
