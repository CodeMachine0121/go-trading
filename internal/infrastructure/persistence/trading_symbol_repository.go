package persistence

import (
	"context"
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
