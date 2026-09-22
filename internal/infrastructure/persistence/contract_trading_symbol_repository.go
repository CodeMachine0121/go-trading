package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ContractTradingSymbolRepository stores the perpetual contracts the system knows
// about, in PostgreSQL.
//
// It is a separate table from the spot list so that the same name can be registered
// on both venues at once, which it has to be: BTCUSDT is a different instrument on
// each, and the spot list keys on the name alone.
type ContractTradingSymbolRepository struct {
	database *gorm.DB
}

func NewContractTradingSymbolRepository(database *gorm.DB) *ContractTradingSymbolRepository {
	return &ContractTradingSymbolRepository{database: database}
}

// FindAll returns every registered contract, ordered by name.
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) FindAll(
	executionContext context.Context,
) ([]entities.ContractTradingSymbol, error) {
	contractTradingSymbols := make([]entities.ContractTradingSymbol, 0)

	result := contractTradingSymbolRepository.database.WithContext(executionContext).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Find(&contractTradingSymbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract trading symbols: %w", result.Error)
	}

	return contractTradingSymbols, nil
}

// FindWatched returns the contracts the system is keeping up to date, ordered by
// name. Name alone settles the order because this list has no follow places to hand
// out, so nothing here depends on which contract was registered first.
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) FindWatched(
	executionContext context.Context,
) ([]entities.ContractTradingSymbol, error) {
	watchedSymbols := make([]entities.ContractTradingSymbol, 0)

	result := contractTradingSymbolRepository.database.WithContext(executionContext).
		Where(&entities.ContractTradingSymbol{IsWatched: true}, "IsWatched").
		Order(clause.OrderByColumn{Column: clause.Column{Name: "symbol"}}).
		Find(&watchedSymbols)
	if result.Error != nil {
		return nil, fmt.Errorf("find watched contract trading symbols: %w", result.Error)
	}

	return watchedSymbols, nil
}

// FindBySymbol answers with the registered contract carrying this name, and whether
// there is one.
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) FindBySymbol(
	executionContext context.Context, symbol string,
) (entities.ContractTradingSymbol, bool, error) {
	contractTradingSymbol := entities.ContractTradingSymbol{}

	found := contractTradingSymbolRepository.database.WithContext(executionContext).
		Where(&entities.ContractTradingSymbol{Symbol: symbol}).
		First(&contractTradingSymbol)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.ContractTradingSymbol{}, false, nil
	}
	if found.Error != nil {
		return entities.ContractTradingSymbol{}, false,
			fmt.Errorf("find contract trading symbol: %w", found.Error)
	}

	return contractTradingSymbol, true, nil
}

// Save registers a contract or updates the one already registered under that name.
//
// The watched flag is named explicitly so that switching it off is stored: false is
// an empty value to the ORM, and left to its own reading it would take "stop
// following this" to mean "change nothing".
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) Save(
	executionContext context.Context, contractTradingSymbol entities.ContractTradingSymbol,
) error {
	result := contractTradingSymbolRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}},
			DoUpdates: clause.AssignmentColumns([]string{"is_watched"}),
		}).
		Create(&contractTradingSymbol)
	if result.Error != nil {
		return fmt.Errorf("save contract trading symbol: %w", result.Error)
	}

	return nil
}
