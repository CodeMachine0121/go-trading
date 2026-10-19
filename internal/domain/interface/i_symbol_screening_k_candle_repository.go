package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_symbol_screening_k_candle_repository.go -destination=mocks/mock_i_symbol_screening_k_candle_repository.go -package=mocks

type ISymbolScreeningKCandleRepository interface {
	// FindInRange includes the start and excludes the end, oldest first.
	FindInRange(
		executionContext context.Context, symbol string, interval string, startTime time.Time, endTime time.Time,
	) ([]entities.SymbolScreeningKCandle, error)
	// SaveAll replaces candles already kept for the same symbol, interval and open time.
	SaveAll(executionContext context.Context, kCandles []entities.SymbolScreeningKCandle) error
	DeleteFetchedBefore(executionContext context.Context, cutoff time.Time) (int, error)
}
