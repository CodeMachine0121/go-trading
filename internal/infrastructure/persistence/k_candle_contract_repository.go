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

// contractFigureColumns are named explicitly so zero volume and trade count are written rather than skipped.
var contractFigureColumns = []string{
	"open", "high", "low", "close",
	"volume", "quote_volume", "taker_buy_base_volume", "taker_buy_quote_volume",
	"trade_count",
	"mark_open", "mark_high", "mark_low", "mark_close",
	"index_open", "index_high", "index_low", "index_close",
	"premium_index_open", "premium_index_high", "premium_index_low", "premium_index_close",
}

// laterLineColumns were added after candles were already stored, so they are the only columns a write may fill on an existing candle.
var laterLineColumns = []string{
	"index_open", "index_high", "index_low", "index_close",
	"premium_index_open", "premium_index_high", "premium_index_low", "premium_index_close",
}

// KCandleContractRepository uses its own table so contract and spot candles with the same symbol and minute do not overwrite each other.
type KCandleContractRepository struct {
	database *gorm.DB
}

func NewKCandleContractRepository(database *gorm.DB) *KCandleContractRepository {
	return &KCandleContractRepository{database: database}
}

func (kCandleContractRepository *KCandleContractRepository) Save(
	executionContext context.Context, kCandleContract entities.KCandleContract,
) (entities.KCandleContract, error) {
	result := kCandleContractRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoUpdates: clause.AssignmentColumns(contractFigureColumns),
		}).
		Create(&kCandleContract)
	if result.Error != nil {
		return entities.KCandleContract{}, fmt.Errorf("save contract k candle: %w", result.Error)
	}

	return kCandleContract, nil
}

// SaveAllIfAbsent inserts new candles in one statement and fills in index and premium lines on old candles that lack them without touching other figures, counting both; an empty input skips the store because the driver rejects empty inserts.
func (kCandleContractRepository *KCandleContractRepository) SaveAllIfAbsent(
	executionContext context.Context, kCandleContracts []entities.KCandleContract,
) (int, error) {
	if len(kCandleContracts) == 0 {
		return 0, nil
	}

	result := kCandleContractRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoUpdates: clause.AssignmentColumns(laterLineColumns),
			Where: clause.Where{Exprs: []clause.Expression{clause.Or(
				clause.Eq{Column: clause.Column{
					Table: entities.KCandleContract{}.TableName(), Name: "index_open"}, Value: nil},
				clause.Eq{Column: clause.Column{
					Table: entities.KCandleContract{}.TableName(), Name: "premium_index_open"}, Value: nil},
			)}},
		}).
		Create(&kCandleContracts)
	if result.Error != nil {
		return 0, fmt.Errorf("save contract k candles if absent: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// CountInRange counts complete candles in the inclusive range, excluding old ones missing the index and premium lines, since it decides whether a day is complete.
func (kCandleContractRepository *KCandleContractRepository) CountInRange(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) (int, error) {
	heldCount := int64(0)

	result := kCandleContractRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContract{}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Gte{Column: "open_time", Value: startTime.UTC()}).
		Where(clause.Lte{Column: "open_time", Value: endTime.UTC()}).
		Where(clause.Neq{Column: "index_open", Value: nil}).
		Where(clause.Neq{Column: "premium_index_open", Value: nil}).
		Count(&heldCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count contract k candles: %w", result.Error)
	}

	return int(heldCount), nil
}

func (kCandleContractRepository *KCandleContractRepository) Update(
	executionContext context.Context, kCandleContract entities.KCandleContract,
) (entities.KCandleContract, error) {
	result := kCandleContractRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContract{}).
		Where(&entities.KCandleContract{
			Symbol: kCandleContract.Symbol, OpenTime: kCandleContract.OpenTime,
		}).
		Select(contractFigureColumns).
		Updates(kCandleContract)
	if result.Error != nil {
		return entities.KCandleContract{}, fmt.Errorf("update contract k candle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return entities.KCandleContract{}, domains.ErrKCandleContractNotFound
	}

	return kCandleContract, nil
}

func (kCandleContractRepository *KCandleContractRepository) FindOne(
	executionContext context.Context, symbol string, openTime time.Time,
) (entities.KCandleContract, error) {
	var kCandleContract entities.KCandleContract

	result := kCandleContractRepository.database.WithContext(executionContext).
		Where(&entities.KCandleContract{Symbol: symbol, OpenTime: openTime}).
		First(&kCandleContract)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.KCandleContract{}, domains.ErrKCandleContractNotFound
	}
	if result.Error != nil {
		return entities.KCandleContract{}, fmt.Errorf("find contract k candle: %w", result.Error)
	}

	return kCandleContract, nil
}

// FindInRange returns at most limit candles in the inclusive range, earliest first.
func (kCandleContractRepository *KCandleContractRepository) FindInRange(
	executionContext context.Context, query domains.KCandleQueryDomain, limit int,
) ([]entities.KCandleContract, error) {
	kCandleContracts := make([]entities.KCandleContract, 0, min(limit, preallocationCeiling))

	result := kCandleContractRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: query.Symbol()},
			clause.Gte{Column: clause.Column{Name: "open_time"}, Value: query.StartTime()},
			clause.Lte{Column: clause.Column{Name: "open_time"}, Value: query.EndTime()},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}}).
		Limit(limit).
		Find(&kCandleContracts)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract k candles in range: %w", result.Error)
	}

	return kCandleContracts, nil
}

func (kCandleContractRepository *KCandleContractRepository) FindDistinctSymbols(
	executionContext context.Context,
) ([]string, error) {
	symbols := make([]string, 0)

	result := kCandleContractRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContract{}).
		Distinct().
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Pluck("symbol", &symbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find distinct contract k candle symbols: %w", result.Error)
	}

	return symbols, nil
}

// FindLatest returns at most limit candles, newest first.
func (kCandleContractRepository *KCandleContractRepository) FindLatest(
	executionContext context.Context, symbol string, limit int,
) ([]entities.KCandleContract, error) {
	kCandleContracts := make([]entities.KCandleContract, 0, min(limit, preallocationCeiling))

	result := kCandleContractRepository.database.WithContext(executionContext).
		Where(&entities.KCandleContract{Symbol: symbol}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}, Desc: true}).
		Limit(limit).
		Find(&kCandleContracts)
	if result.Error != nil {
		return nil, fmt.Errorf("find latest contract k candles: %w", result.Error)
	}

	return kCandleContracts, nil
}

// FindLatestBefore returns at most limit candles strictly before the cut-off, newest first.
func (kCandleContractRepository *KCandleContractRepository) FindLatestBefore(
	executionContext context.Context, symbol string, cutoffTime time.Time, limit int,
) ([]entities.KCandleContract, error) {
	kCandleContracts := make([]entities.KCandleContract, 0, min(limit, preallocationCeiling))

	result := kCandleContractRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: symbol},
			clause.Lt{Column: clause.Column{Name: "open_time"}, Value: cutoffTime},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "open_time"}, Desc: true}).
		Limit(limit).
		Find(&kCandleContracts)
	if result.Error != nil {
		return nil, fmt.Errorf("find latest contract k candles before cutoff: %w", result.Error)
	}

	return kCandleContracts, nil
}

func (kCandleContractRepository *KCandleContractRepository) Delete(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	result := kCandleContractRepository.database.WithContext(executionContext).
		Where(&entities.KCandleContract{Symbol: symbol, OpenTime: openTime}).
		Delete(&entities.KCandleContract{})
	if result.Error != nil {
		return fmt.Errorf("delete contract k candle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.ErrKCandleContractNotFound
	}

	return nil
}
