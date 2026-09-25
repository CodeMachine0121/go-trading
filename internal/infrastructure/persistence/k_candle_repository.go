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

// preallocationCeiling caps upfront slice capacity, since aggregated read limits can reach hundreds of thousands even over an empty table.
const preallocationCeiling = 1000

// figureColumns are named explicitly so zero figures are written rather than skipped.
var figureColumns = []string{
	"open", "high", "low", "close",
	"volume", "quote_volume", "taker_buy_base_volume", "taker_buy_quote_volume",
}

type KCandleRepository struct {
	database *gorm.DB
}

func NewKCandleRepository(database *gorm.DB) *KCandleRepository {
	return &KCandleRepository{database: database}
}

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

// SaveAllIfAbsent inserts new candles in one statement (per-row round trips dominate multi-year backfills) and returns the count stored; an empty input skips the store because the driver rejects empty inserts.
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

// CountInRange counts candles in the inclusive range.
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

// FindInRange returns at most limit candles in the inclusive range, earliest first.
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

// FindDistinctSymbols lets the database both de-duplicate and order.
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

// FindLatest returns at most limit candles, newest first; reversing is the caller's concern.
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

// FindLatestBefore returns at most limit candles strictly before the cut-off, newest first, so a cut-off on a bucket edge reads the bucket ending there.
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
