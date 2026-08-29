package persistence

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
func (kCandleRepository *KCandleRepository) Save(kCandle entities.KCandle) (entities.KCandle, error) {
	result := kCandleRepository.database.
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
func (kCandleRepository *KCandleRepository) Update(kCandle entities.KCandle) (entities.KCandle, error) {
	result := kCandleRepository.database.
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
func (kCandleRepository *KCandleRepository) FindOne(symbol string, openTime time.Time) (entities.KCandle, error) {
	var kCandle entities.KCandle

	result := kCandleRepository.database.
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
	query domains.KCandleQueryDomain, limit int,
) ([]entities.KCandle, error) {
	kCandles := make([]entities.KCandle, 0, limit)

	result := kCandleRepository.database.
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

// FindLatest returns at most limit K candles for the trading symbol, newest first.
// The order is deliberately the opposite of FindInRange: reading "the latest few"
// is a descending query, and turning the result the right way round is the
// caller's business rule, not this repository's.
func (kCandleRepository *KCandleRepository) FindLatest(
	symbol string, limit int,
) ([]entities.KCandle, error) {
	kCandles := make([]entities.KCandle, 0, limit)

	result := kCandleRepository.database.
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
func (kCandleRepository *KCandleRepository) Delete(symbol string, openTime time.Time) error {
	result := kCandleRepository.database.
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
