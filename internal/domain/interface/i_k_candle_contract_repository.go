package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_contract_repository.go -destination=mocks/mock_i_k_candle_contract_repository.go -package=mocks

// IKCandleContractRepository stores and retrieves perpetual contract K candles.
//
// It is a separate contract from IKCandleRepository, not the same one pointed at
// another table, because the record it carries is a different record: a contract
// candle without its mark price is not one, and a spot candle has no mark price to
// give. Sharing the interface would mean handing one side a shape the other cannot
// fill.
//
// Every method locates a candle by trading symbol alone. There is no market argument
// and there does not need to be: an instance already knows which venue it reads.
type IKCandleContractRepository interface {
	// Save stores a contract K candle, replacing the figures of any candle already
	// held for the same trading symbol and open time.
	Save(
		executionContext context.Context, kCandleContract entities.KCandleContract,
	) (entities.KCandleContract, error)
	// SaveAllIfAbsent stores every contract K candle nothing is held for yet, and
	// says how many of them it actually stored. Finding one already there is an
	// ordinary outcome, not a failure — it is what lets an interrupted history sync
	// be re-run from where it stopped.
	//
	// A candle held from before the index price and premium index existed counts as
	// absent for those two lines only: they are filled in, and every figure it
	// already held is left as it was.
	SaveAllIfAbsent(
		executionContext context.Context, kCandleContracts []entities.KCandleContract,
	) (int, error)
	// CountInRange is how many complete contract K candles are held for this symbol
	// across the stretch, both ends included — a candle stored before the index price
	// and premium index existed is not complete, and is not counted.
	//
	// It is what decides whether a day of a history sync is asked for at all: a day
	// already holding every minute it could is never asked about again.
	//
	// **It counts what is stored, which is not always what the venue can give.** A
	// minute the venue has traded figures but no mark, index or premium line for is one this system
	// will never store, so a day containing one can never reach the count a full day
	// would — and is therefore re-fetched by every later sync over that stretch. That
	// is the whole of the cost: nothing is lost or corrupted, the same day is simply
	// asked about again. It is real at the far end of a contract's life, where mark
	// price history starts later than candle history.
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
	// FindDistinctSymbols returns every trading symbol that has at least one stored
	// contract K candle, each once, ordered by name.
	FindDistinctSymbols(executionContext context.Context) ([]string, error)
	// FindLatest returns at most limit contract K candles for the symbol, ordered by
	// open time NEWEST FIRST. It is what a backfill reads to find where it may resume.
	FindLatest(
		executionContext context.Context, symbol string, limit int,
	) ([]entities.KCandleContract, error)
	Delete(executionContext context.Context, symbol string, openTime time.Time) error
}
