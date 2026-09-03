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
