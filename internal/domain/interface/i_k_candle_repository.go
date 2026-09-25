package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_repository.go -destination=mocks/mock_i_k_candle_repository.go -package=mocks

// IKCandleRepository stores K candles; Save overwrites any candle with the same symbol and open time.
type IKCandleRepository interface {
	Save(executionContext context.Context, kCandle entities.KCandle) (entities.KCandle, error)
	// SaveAllIfAbsent batch-inserts only missing candles and returns how many were stored, never touching held ones (unlike Save, which replaces forming candles).
	// Batching exists because history is fetched a day (1000+ candles) at a time; an empty batch is a valid no-op.
	SaveAllIfAbsent(executionContext context.Context, kCandles []entities.KCandle) (int, error)
	// CountInRange includes both ends; a sync compares it with the expected count to skip already-complete days.
	CountInRange(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) (int, error)
	Update(executionContext context.Context, kCandle entities.KCandle) (entities.KCandle, error)
	FindOne(executionContext context.Context, symbol string, openTime time.Time) (entities.KCandle, error)
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.KCandle, error)
	// FindDistinctSymbols returns each symbol with stored candles once, ordered by name.
	FindDistinctSymbols(executionContext context.Context) ([]string, error)
	// FindLatest returns newest first, the opposite order to FindInRange.
	FindLatest(executionContext context.Context, symbol string, limit int) ([]entities.KCandle, error)
	// FindLatestBefore returns candles strictly before cutoffTime, newest first.
	FindLatestBefore(
		executionContext context.Context, symbol string, cutoffTime time.Time, limit int,
	) ([]entities.KCandle, error)
	Delete(executionContext context.Context, symbol string, openTime time.Time) error
}
