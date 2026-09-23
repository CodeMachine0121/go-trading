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
	// A contract carrying no specification leaves the one held alone: saving "now
	// watched" or "no longer watched" is not saying the specification went away.
	writtenColumns := []string{"is_watched"}
	if contractTradingSymbol.SpecificationUpdatedAt != nil {
		writtenColumns = append(writtenColumns, tradingSpecificationColumns...)
	}

	result := contractTradingSymbolRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}},
			DoUpdates: clause.AssignmentColumns(writtenColumns),
		}).
		Create(&contractTradingSymbol)
	if result.Error != nil {
		return fmt.Errorf("save contract trading symbol: %w", result.Error)
	}

	return nil
}

// tradingSpecificationColumns are the columns a specification refresh writes, listed
// so that nothing else about a contract — whether it is watched — is touched by one.
var tradingSpecificationColumns = []string{
	"tick_size", "quantity_step", "minimum_quantity", "minimum_notional",
	"maintenance_margin_rate", "liquidation_fee_rate", "funding_interval_hours",
	"specification_updated_at",
}

// SaveTradingSpecifications writes each contract's specification in one transaction,
// so a refresh is either recorded for every contract it covered or for none of them.
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) SaveTradingSpecifications(
	executionContext context.Context, contractTradingSymbols []entities.ContractTradingSymbol,
) error {
	transactionError := contractTradingSymbolRepository.database.WithContext(executionContext).
		Transaction(func(transaction *gorm.DB) error {
			for _, contractTradingSymbol := range contractTradingSymbols {
				result := transaction.Model(&entities.ContractTradingSymbol{}).
					Where(&entities.ContractTradingSymbol{Symbol: contractTradingSymbol.Symbol}).
					Select(tradingSpecificationColumns).
					Updates(contractTradingSymbol)
				if result.Error != nil {
					return result.Error
				}
			}

			return nil
		})
	if transactionError != nil {
		return fmt.Errorf("save contract trading specifications: %w", transactionError)
	}

	return nil
}
