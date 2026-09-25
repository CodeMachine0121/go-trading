package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ContractTradingSymbolRepository uses a table separate from spot symbols, since the same name (e.g. BTCUSDT) is a different instrument on each venue.
type ContractTradingSymbolRepository struct {
	database *gorm.DB
}

func NewContractTradingSymbolRepository(database *gorm.DB) *ContractTradingSymbolRepository {
	return &ContractTradingSymbolRepository{database: database}
}

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

// FindWatched orders by name only, since contracts have no follow positions.
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

// Save upserts by name, naming the watched column explicitly so false is written instead of skipped as a zero value.
func (contractTradingSymbolRepository *ContractTradingSymbolRepository) Save(
	executionContext context.Context, contractTradingSymbol entities.ContractTradingSymbol,
) error {
	// A contract without a specification leaves the stored one untouched.
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

// tradingSpecificationColumns lists the columns a refresh writes, so the watched flag is never touched.
var tradingSpecificationColumns = []string{
	"tick_size", "quantity_step", "minimum_quantity", "minimum_notional",
	"maintenance_margin_rate", "liquidation_fee_rate", "funding_interval_hours",
	"specification_updated_at",
}

// SaveTradingSpecifications writes all specifications in one transaction.
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
