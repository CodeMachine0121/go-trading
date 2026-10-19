package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const symbolScreeningSaveBatchSize = 500

type SymbolScreeningKCandleRepository struct {
	database *gorm.DB
}

func NewSymbolScreeningKCandleRepository(database *gorm.DB) *SymbolScreeningKCandleRepository {
	return &SymbolScreeningKCandleRepository{database: database}
}

func (kCandleRepository *SymbolScreeningKCandleRepository) FindInRange(
	executionContext context.Context, symbol string, interval string, startTime time.Time, endTime time.Time,
) ([]entities.SymbolScreeningKCandle, error) {
	kCandles := []entities.SymbolScreeningKCandle{}
	found := kCandleRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Eq{Column: "interval", Value: interval}).
		Where(clause.Gte{Column: "open_time", Value: startTime.UTC()}).
		Where(clause.Lt{Column: "open_time", Value: endTime.UTC()}).
		Order("open_time ASC").
		Find(&kCandles)
	if found.Error != nil {
		return nil, fmt.Errorf("find symbol screening k candles: %w", found.Error)
	}

	return kCandles, nil
}

func (kCandleRepository *SymbolScreeningKCandleRepository) SaveAll(
	executionContext context.Context, kCandles []entities.SymbolScreeningKCandle,
) error {
	if len(kCandles) == 0 {
		return nil
	}
	saved := kCandleRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "symbol"}, {Name: "interval"}, {Name: "open_time"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"open", "high", "low", "close", "volume", "quote_volume", "taker_buy_base_volume",
				"taker_buy_quote_volume", "trade_count", "mark_open", "mark_high", "mark_low", "mark_close", "fetched_at",
			}),
		}).
		CreateInBatches(&kCandles, symbolScreeningSaveBatchSize)
	if saved.Error != nil {
		return fmt.Errorf("save symbol screening k candles: %w", saved.Error)
	}

	return nil
}

func (kCandleRepository *SymbolScreeningKCandleRepository) DeleteFetchedBefore(
	executionContext context.Context, cutoff time.Time,
) (int, error) {
	deleted := kCandleRepository.database.WithContext(executionContext).
		Where(clause.Lt{Column: "fetched_at", Value: cutoff.UTC()}).
		Delete(&entities.SymbolScreeningKCandle{})
	if deleted.Error != nil {
		return 0, fmt.Errorf("delete expired symbol screening k candles: %w", deleted.Error)
	}

	return int(deleted.RowsAffected), nil
}
