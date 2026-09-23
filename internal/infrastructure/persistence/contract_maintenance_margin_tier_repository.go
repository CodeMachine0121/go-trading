package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ContractMaintenanceMarginTierRepository stores maintenance margin ladders in
// PostgreSQL.
type ContractMaintenanceMarginTierRepository struct {
	database *gorm.DB
}

func NewContractMaintenanceMarginTierRepository(database *gorm.DB) *ContractMaintenanceMarginTierRepository {
	return &ContractMaintenanceMarginTierRepository{database: database}
}

// ReplaceLadders swaps each named contract's ladder for the one given, in a single
// transaction. A ladder is removed and written back rather than merged, because a new
// ladder with one tier fewer means that tier no longer exists.
func (tierRepository *ContractMaintenanceMarginTierRepository) ReplaceLadders(
	executionContext context.Context, laddersBySymbol map[string][]entities.ContractMaintenanceMarginTier,
) error {
	transactionError := tierRepository.database.WithContext(executionContext).
		Transaction(func(transaction *gorm.DB) error {
			for symbol, tiers := range laddersBySymbol {
				if deleteError := transaction.
					Where(&entities.ContractMaintenanceMarginTier{Symbol: symbol}).
					Delete(&entities.ContractMaintenanceMarginTier{}).Error; deleteError != nil {
					return deleteError
				}
				if len(tiers) == 0 {
					continue
				}
				if createError := transaction.Create(&tiers).Error; createError != nil {
					return createError
				}
			}

			return nil
		})
	if transactionError != nil {
		return fmt.Errorf("replace contract maintenance margin ladders: %w", transactionError)
	}

	return nil
}

// FindBySymbol is one contract's ladder, first tier first.
func (tierRepository *ContractMaintenanceMarginTierRepository) FindBySymbol(
	executionContext context.Context, symbol string,
) ([]entities.ContractMaintenanceMarginTier, error) {
	tiers := make([]entities.ContractMaintenanceMarginTier, 0)

	result := tierRepository.database.WithContext(executionContext).
		Where(&entities.ContractMaintenanceMarginTier{Symbol: symbol}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "tier"}}).
		Find(&tiers)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract maintenance margin ladder: %w", result.Error)
	}

	return tiers, nil
}
