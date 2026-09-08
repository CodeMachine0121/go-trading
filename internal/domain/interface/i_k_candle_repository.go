package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_repository.go -destination=mocks/mock_i_k_candle_repository.go -package=mocks

// IKCandleRepository stores and retrieves K candles. Save carries the overwrite
// semantics: storing a candle whose symbol and open time already exist replaces it.
type IKCandleRepository interface {
	Save(executionContext context.Context, kCandle entities.KCandle) (entities.KCandle, error)
	Update(executionContext context.Context, kCandle entities.KCandle) (entities.KCandle, error)
	FindOne(executionContext context.Context, symbol string, openTime time.Time) (entities.KCandle, error)
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.KCandle, error)
	// FindDistinctSymbols returns every trading symbol that has at least one stored K
	// candle, each once, ordered by name. It hands back the column's values rather
	// than K candles: a K candle carrying nothing but a symbol is easy to mistake for
	// a real one.
	FindDistinctSymbols(executionContext context.Context) ([]string, error)
	// FindLatest returns at most limit K candles for the symbol, ordered by open
	// time NEWEST FIRST. Note this is the opposite order to FindInRange, because
	// "the latest few" is naturally a descending read.
	FindLatest(executionContext context.Context, symbol string, limit int) ([]entities.KCandle, error)
	// FindLatestBefore returns at most limit K candles for the symbol whose open
	// time is STRICTLY BEFORE cutoffTime, newest first.
	//
	// It is not FindLatest with one more argument. "The latest few" and "the latest
	// few as of a moment" are different questions, and the caller asking the first
	// has no moment to name — handing it a cut-off far in the future would make that
	// call look like it were filtering by time when it is not.
	FindLatestBefore(
		executionContext context.Context, symbol string, cutoffTime time.Time, limit int,
	) ([]entities.KCandle, error)
	Delete(executionContext context.Context, symbol string, openTime time.Time) error
}
