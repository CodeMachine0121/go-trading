package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_live_k_candle_snapshot_repository.go -destination=mocks/mock_i_live_k_candle_snapshot_repository.go -package=mocks

// ILiveKCandleSnapshotRepository holds the latest live candle per rostered symbol, written by the replica on duty and read by the rest.
type ILiveKCandleSnapshotRepository interface {
	// Save replaces the symbol's snapshot.
	Save(executionContext context.Context, snapshot entities.LiveKCandleSnapshot) error

	FindBySymbols(executionContext context.Context, symbols []string) ([]entities.LiveKCandleSnapshot, error)
}
