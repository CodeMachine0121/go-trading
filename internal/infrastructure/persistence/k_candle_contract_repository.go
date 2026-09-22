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

// contractFigureColumns are the columns a write always sets, listed explicitly so
// that a figure of zero is stored rather than skipped as an empty value. A minute in
// which nothing traded has a volume and a trade count of zero, and both are facts.
var contractFigureColumns = []string{
	"open", "high", "low", "close",
	"volume", "quote_volume", "taker_buy_base_volume", "taker_buy_quote_volume",
	"trade_count",
	"mark_open", "mark_high", "mark_low", "mark_close",
}

// KCandleContractRepository stores perpetual contract K candles in PostgreSQL.
//
// It writes to its own table, which is what keeps a contract candle and a spot candle
// for the same symbol and minute from overwriting one another. Both carry the same
// unique key, so a shared table would have the automatic rounds quietly replacing
// each other's rows every minute with nothing to complain about.
type KCandleContractRepository struct {
	database *gorm.DB
}

func NewKCandleContractRepository(database *gorm.DB) *KCandleContractRepository {
	return &KCandleContractRepository{database: database}
}

// Save stores a contract K candle, replacing the figures of any candle already held
// for the same trading symbol and open time.
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

// SaveAllIfAbsent stores every contract K candle nothing is held for yet, in one
// statement, and says how many it stored.
//
// An empty batch is answered without touching the store at all, because the driver
// refuses a statement with no rows and a stretch the contract did not yet exist over
// legitimately produces one.
func (kCandleContractRepository *KCandleContractRepository) SaveAllIfAbsent(
	executionContext context.Context, kCandleContracts []entities.KCandleContract,
) (int, error) {
	if len(kCandleContracts) == 0 {
		return 0, nil
	}

	result := kCandleContractRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "open_time"}},
			DoNothing: true,
		}).
		Create(&kCandleContracts)
	if result.Error != nil {
		return 0, fmt.Errorf("save contract k candles if absent: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// CountInRange is how many contract K candles are held for this symbol across the
// stretch, both ends included.
func (kCandleContractRepository *KCandleContractRepository) CountInRange(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) (int, error) {
	heldCount := int64(0)

	result := kCandleContractRepository.database.WithContext(executionContext).
		Model(&entities.KCandleContract{}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Gte{Column: "open_time", Value: startTime.UTC()}).
		Where(clause.Lte{Column: "open_time", Value: endTime.UTC()}).
		Count(&heldCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count contract k candles: %w", result.Error)
	}

	return int(heldCount), nil
}

// Update replaces the figures of an existing contract K candle, reporting not found
// when the trading symbol and open time name no candle.
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

// FindOne returns the contract K candle named by trading symbol and open time.
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

// FindInRange returns at most limit contract K candles whose open time falls inside
// the query's range, both ends included, earliest first.
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

// FindDistinctSymbols returns every trading symbol that has at least one stored
// contract K candle, each once, ordered by name.
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

// FindLatest returns at most limit contract K candles for the trading symbol, newest
// first. The order is deliberately the opposite of FindInRange: reading "the latest
// few" is a descending query.
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

// Delete removes the contract K candle named by trading symbol and open time,
// reporting not found when it names no candle.
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
