package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// preallocationCeiling bounds how much room a read reserves before it has seen a
// single row. A read limit is an upper bound on what the data *could* hold, and an
// aggregated query's bound reaches into the hundreds of thousands; reserving that up
// front would let one request over an empty database claim tens of megabytes. The
// slice still grows to hold whatever actually arrives — this only stops the guess
// from being the expensive part.
const preallocationCeiling = 1000

// figureColumns are the columns a write always sets, listed explicitly so that a
// figure of zero is stored rather than skipped as an empty value.
var figureColumns = []string{
	"open", "high", "low", "close",
	"volume", "quote_volume", "taker_buy_base_volume", "taker_buy_quote_volume",
}

// KCandleRepository stores K candles in PostgreSQL.
type KCandleRepository struct {
	database *gorm.DB
}

func NewKCandleRepository(database *gorm.DB) *KCandleRepository {
	return &KCandleRepository{database: database}
}

// Save stores a K candle, replacing the figures of any candle already held for the
// same trading symbol and open time.
func (kCandleRepository *KCandleRepository) Save(
	executionContext context.Context, kCandle entities.KCandle,
) (entities.KCandle, error) {
	result := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoUpdates: clause.AssignmentColumns(figureColumns),
		}).
		Create(&kCandle)
	if result.Error != nil {
		return entities.KCandle{}, fmt.Errorf("save k candle: %w", result.Error)
	}

	return kCandle, nil
}

// SaveIfAbsent stores a K candle only when none is held for that trading symbol and
// open time, and says whether it stored one.
//
// The store decides, not a read followed by a write: two fetches of overlapping
// stretches running at once would both find a minute absent and both go on to write
// it, and the second would overwrite exactly what this exists to protect.
func (kCandleRepository *KCandleRepository) SaveIfAbsent(
	executionContext context.Context, kCandle entities.KCandle,
) (bool, error) {
	result := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoNothing: true,
		}).
		Create(&kCandle)
	if result.Error != nil {
		return false, fmt.Errorf("save k candle if absent: %w", result.Error)
	}

	return result.RowsAffected > 0, nil
}

// SaveAllIfAbsent stores every K candle nothing is held for yet, in one statement,
// and says how many it stored.
//
// One statement rather than one per candle: a day of a round-the-clock market is over
// a thousand of them, and four years is two million — at which point the round trips
// are most of what the run costs.
//
// An empty batch is answered without touching the store at all, because the driver
// refuses a statement with no rows and a day the market was shut on legitimately
// produces one.
func (kCandleRepository *KCandleRepository) SaveAllIfAbsent(
	executionContext context.Context, kCandles []entities.KCandle,
) (int, error) {
	if len(kCandles) == 0 {
		return 0, nil
	}

	result := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoNothing: true,
		}).
		Create(&kCandles)
	if result.Error != nil {
		return 0, fmt.Errorf("save k candles if absent: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// CountInRange is how many K candles are held for this symbol across the stretch,
// both ends included.
func (kCandleRepository *KCandleRepository) CountInRange(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) (int, error) {
	heldCount := int64(0)

	result := kCandleRepository.database.WithContext(executionContext).
		Model(&entities.KCandle{}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Gte{Column: "open_time", Value: startTime.UTC()}).
		Where(clause.Lte{Column: "open_time", Value: endTime.UTC()}).
		Count(&heldCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count k candles: %w", result.Error)
	}

	return int(heldCount), nil
}

// Update replaces the figures of an existing K candle, reporting not found when the
// trading symbol and open time name no candle.
func (kCandleRepository *KCandleRepository) Update(
	executionContext context.Context, kCandle entities.KCandle,
) (entities.KCandle, error) {
	result := kCandleRepository.database.WithContext(executionContext).
		Model(&entities.KCandle{}).
		Where(&entities.KCandle{Symbol: kCandle.Symbol, OpenTime: kCandle.OpenTime}).
		Select(figureColumns).
		Updates(kCandle)
	if result.Error != nil {
		return entities.KCandle{}, fmt.Errorf("update k candle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return entities.KCandle{}, domains.ErrKCandleNotFound
	}

	return kCandle, nil
}

// FindOne returns the K candle named by trading symbol and open time.
func (kCandleRepository *KCandleRepository) FindOne(
	executionContext context.Context, symbol string, openTime time.Time,
) (entities.KCandle, error) {
	var kCandle entities.KCandle

	result := kCandleRepository.database.WithContext(executionContext).
		Where(&entities.KCandle{Symbol: symbol, OpenTime: openTime}).
		First(&kCandle)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.KCandle{}, domains.ErrKCandleNotFound
	}
	if result.Error != nil {
		return entities.KCandle{}, fmt.Errorf("find k candle: %w", result.Error)
	}

	return kCandle, nil
}

// FindInRange returns at most limit K candles whose open time falls inside the
// query's range, both ends included, earliest first.
func (kCandleRepository *KCandleRepository) FindInRange(
	executionContext context.Context, query domains.KCandleQueryDomain, limit int,
) ([]entities.KCandle, error) {
	kCandles := make([]entities.KCandle, 0, min(limit, preallocationCeiling))

	result := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: query.Symbol()},
			clause.Gte{Column: clause.Column{Name: "open_time"}, Value: query.StartTime()},
			clause.Lte{Column: clause.Column{Name: "open_time"}, Value: query.EndTime()},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}}).
		Limit(limit).
		Find(&kCandles)
	if result.Error != nil {
		return nil, fmt.Errorf("find k candles in range: %w", result.Error)
	}

	return kCandles, nil
}

// FindDistinctSymbols returns every trading symbol that has at least one stored K
// candle, each once, ordered by name. Both the de-duplication and the ordering are
// the database's job: doing either of them again in Go would give the two places a
// chance to disagree.
func (kCandleRepository *KCandleRepository) FindDistinctSymbols(executionContext context.Context) ([]string, error) {
	symbols := make([]string, 0)

	result := kCandleRepository.database.WithContext(executionContext).
		Model(&entities.KCandle{}).
		Distinct().
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Pluck("symbol", &symbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find distinct k candle symbols: %w", result.Error)
	}

	return symbols, nil
}

// FindLatest returns at most limit K candles for the trading symbol, newest first.
// The order is deliberately the opposite of FindInRange: reading "the latest few"
// is a descending query, and turning the result the right way round is the
// caller's business rule, not this repository's.
func (kCandleRepository *KCandleRepository) FindLatest(
	executionContext context.Context, symbol string, limit int,
) ([]entities.KCandle, error) {
	kCandles := make([]entities.KCandle, 0, min(limit, preallocationCeiling))

	result := kCandleRepository.database.WithContext(executionContext).
		Where(&entities.KCandle{Symbol: symbol}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}, Desc: true}).
		Limit(limit).
		Find(&kCandles)
	if result.Error != nil {
		return nil, fmt.Errorf("find latest k candles: %w", result.Error)
	}

	return kCandles, nil
}

// FindLatestBefore returns at most limit K candles for the trading symbol that
// opened strictly before the cut-off, newest first. Strictly before is what makes a
// cut-off on a bucket edge read the bucket that ends there and not the one that
// starts there.
func (kCandleRepository *KCandleRepository) FindLatestBefore(
	executionContext context.Context, symbol string, cutoffTime time.Time, limit int,
) ([]entities.KCandle, error) {
	kCandles := make([]entities.KCandle, 0, min(limit, preallocationCeiling))

	result := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: symbol},
			clause.Lt{Column: clause.Column{Name: "open_time"}, Value: cutoffTime},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}, Desc: true}).
		Limit(limit).
		Find(&kCandles)
	if result.Error != nil {
		return nil, fmt.Errorf("find latest k candles before cutoff: %w", result.Error)
	}

	return kCandles, nil
}

// Delete removes the K candle named by trading symbol and open time, reporting not
// found when it names no candle.
func (kCandleRepository *KCandleRepository) Delete(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	result := kCandleRepository.database.WithContext(executionContext).
		Where(&entities.KCandle{Symbol: symbol, OpenTime: openTime}).
		Delete(&entities.KCandle{})
	if result.Error != nil {
		return fmt.Errorf("delete k candle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.ErrKCandleNotFound
	}

	return nil
}
