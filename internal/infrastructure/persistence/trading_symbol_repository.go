package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradingSymbolRepository struct {
	database *gorm.DB
}

func NewTradingSymbolRepository(database *gorm.DB) *TradingSymbolRepository {
	return &TradingSymbolRepository{database: database}
}

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

// FindWatched orders by registration time with name as tiebreak, since older rows share a zero registration time.
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

// Save upserts by name so registering the same market twice leaves one row.
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

// RegisterAll skips conflicts to tolerate racing migrations; callers still filter out existing symbols first.
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
