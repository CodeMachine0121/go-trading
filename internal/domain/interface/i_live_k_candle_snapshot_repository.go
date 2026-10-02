package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_live_k_candle_snapshot_repository.go -destination=mocks/mock_i_live_k_candle_snapshot_repository.go -package=mocks

// ILiveKCandleSnapshotRepository holds the latest the replica on duty saw of each rostered symbol's minute, read by the rest.
type ILiveKCandleSnapshotRepository interface {
	// Save replaces that symbol's snapshot of that minute.
	Save(executionContext context.Context, snapshot entities.LiveKCandleSnapshot) error

	// FindObservedAfter returns the symbols' snapshots seen after since, oldest minute first, then oldest sighting.
	FindObservedAfter(
		executionContext context.Context, symbols []string, since time.Time,
	) ([]entities.LiveKCandleSnapshot, error)

	// DeleteObservedBefore drops snapshots nobody can still need.
	DeleteObservedBefore(executionContext context.Context, cutoff time.Time) error
}
