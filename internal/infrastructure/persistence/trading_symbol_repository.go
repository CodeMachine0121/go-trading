package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TradingSymbolRepository stores registered trading symbols in PostgreSQL.
type TradingSymbolRepository struct {
	database *gorm.DB
}

func NewTradingSymbolRepository(database *gorm.DB) *TradingSymbolRepository {
	return &TradingSymbolRepository{database: database}
}

// FindAll returns every registered trading symbol, ordered by name.
func (tradingSymbolRepository *TradingSymbolRepository) FindAll(
	executionContext context.Context,
) ([]entities.TradingSymbol, error) {
	tradingSymbols := make([]entities.TradingSymbol, 0)

	result := tradingSymbolRepository.database.WithContext(executionContext).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Find(&tradingSymbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find trading symbols: %w", result.Error)
	}

	return tradingSymbols, nil
}

// FindWatched returns the trading symbols the system is keeping up to date, earliest
// registered first.
//
// Name settles a tie deliberately. Symbols registered before registration time was
// recorded all carry the same zero value, and an order that depends on which row the
// database happens to hand back first would put a different set of symbols in a
// market's follow places on different days.
func (tradingSymbolRepository *TradingSymbolRepository) FindWatched(
	executionContext context.Context,
) ([]entities.TradingSymbol, error) {
	watchedSymbols := make([]entities.TradingSymbol, 0)

	result := tradingSymbolRepository.database.WithContext(executionContext).
		Where(&entities.TradingSymbol{IsWatched: true}, "IsWatched").
		Order(clause.OrderByColumn{Column: clause.Column{Name: "registered_at"}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Find(&watchedSymbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find watched trading symbols: %w", result.Error)
	}

	return watchedSymbols, nil
}

// FindBySymbol returns one registered trading symbol, reporting whether it was
// registered at all.
func (tradingSymbolRepository *TradingSymbolRepository) FindBySymbol(
	executionContext context.Context, symbol string,
) (entities.TradingSymbol, bool, error) {
	tradingSymbol := entities.TradingSymbol{}

	result := tradingSymbolRepository.database.WithContext(executionContext).
		Where(&entities.TradingSymbol{Symbol: symbol}).
		Take(&tradingSymbol)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.TradingSymbol{}, false, nil
	}
	if result.Error != nil {
		return entities.TradingSymbol{}, false, fmt.Errorf("find trading symbol: %w", result.Error)
	}

	return tradingSymbol, true, nil
}

// Save stores one trading symbol, replacing whatever was held under the same name.
//
// Replacing rather than inserting is what makes registering the same market twice
// leave one market: the name is the key, so there is nothing a second row could
// distinguish.
func (tradingSymbolRepository *TradingSymbolRepository) Save(
	executionContext context.Context, tradingSymbol entities.TradingSymbol,
) error {
	result := tradingSymbolRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}},
			UpdateAll: true,
		}).
		Create(&tradingSymbol)
	if result.Error != nil {
		return fmt.Errorf("save trading symbol: %w", result.Error)
	}

	return nil
}

// RegisterAll stores the given trading symbols, leaving any that are already
// registered exactly as they are.
//
// The caller has already worked out which ones are missing; skipping conflicts here
// is not a substitute for that check but a guard against two migrations racing each
// other, where neither should fail over which got there first.
func (tradingSymbolRepository *TradingSymbolRepository) RegisterAll(
	executionContext context.Context, tradingSymbols []entities.TradingSymbol,
) error {
	if len(tradingSymbols) == 0 {
		return nil
	}

	result := tradingSymbolRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&tradingSymbols)
	if result.Error != nil {
		return fmt.Errorf("register trading symbols: %w", result.Error)
	}

	return nil
}
