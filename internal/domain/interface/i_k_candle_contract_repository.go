package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_contract_repository.go -destination=mocks/mock_i_k_candle_contract_repository.go -package=mocks

// IKCandleContractRepository is separate from IKCandleRepository because contract candles require a mark price; instances are venue-bound, so lookups take no market.
type IKCandleContractRepository interface {
	// Save replaces any candle held for the same symbol and open time.
	Save(
		executionContext context.Context, kCandleContract entities.KCandleContract,
	) (entities.KCandleContract, error)
	// SaveAllIfAbsent returns how many were stored, so interrupted syncs can re-run; legacy candles lacking index/premium lines get only those filled in.
	SaveAllIfAbsent(
		executionContext context.Context, kCandleContracts []entities.KCandleContract,
	) (int, error)
	// CountInRange counts only complete candles (both ends included) to decide whether a sync day needs fetching.
	// Minutes the venue lacks mark/index/premium for can never be stored, so such days are harmlessly re-fetched on every sync.
	CountInRange(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) (int, error)
	Update(
		executionContext context.Context, kCandleContract entities.KCandleContract,
	) (entities.KCandleContract, error)
	FindOne(
		executionContext context.Context, symbol string, openTime time.Time,
	) (entities.KCandleContract, error)
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.KCandleContract, error)
	// FindDistinctSymbols returns each symbol with stored candles once, ordered by name.
	FindDistinctSymbols(executionContext context.Context) ([]string, error)
	// FindLatest returns newest first; a backfill uses it to find where to resume.
	FindLatest(
		executionContext context.Context, symbol string, limit int,
	) ([]entities.KCandleContract, error)
	// FindLatestBefore returns candles strictly before cutoffTime, newest first.
	FindLatestBefore(
		executionContext context.Context, symbol string, cutoffTime time.Time, limit int,
	) ([]entities.KCandleContract, error)
	Delete(executionContext context.Context, symbol string, openTime time.Time) error
}
