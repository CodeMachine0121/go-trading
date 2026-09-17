package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_history_sync_run_repository.go -destination=mocks/mock_i_k_candle_history_sync_run_repository.go -package=mocks

// IKCandleHistorySyncRunRepository keeps the history syncs that have been asked for.
//
// A run is written before any candle is fetched and updated as it goes, because the
// work outlives the request that asked for it: a stretch of years takes minutes to
// hours, and the only way for anybody to find out where it got to is for the run to
// be somewhere they can look.
type IKCandleHistorySyncRunRepository interface {
	// Save writes a run, whether it is new or being brought up to date. A new one
	// comes back carrying the identifier it was given.
	Save(
		executionContext context.Context, syncRun entities.KCandleHistorySyncRun,
	) (entities.KCandleHistorySyncRun, error)
	// FindOne answers with the run carrying this identifier, and whether there is one.
	FindOne(
		executionContext context.Context, id uint,
	) (entities.KCandleHistorySyncRun, bool, error)
	// FailAllRunning marks every run still recorded as running as failed. Every one of
	// them is stale by definition: the process that was fetching no longer exists.
	FailAllRunning(executionContext context.Context, reason string) (int, error)
}
