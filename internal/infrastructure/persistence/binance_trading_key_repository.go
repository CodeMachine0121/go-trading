package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BinanceTradingKeyRepository also writes the owner's bots' auto-order switches, because a key change and the switches it invalidates must commit together.
type BinanceTradingKeyRepository struct {
	database *gorm.DB
}

func NewBinanceTradingKeyRepository(database *gorm.DB) *BinanceTradingKeyRepository {
	return &BinanceTradingKeyRepository{database: database}
}

func (binanceTradingKeyRepository *BinanceTradingKeyRepository) FindOneByUser(
	executionContext context.Context, userID uint,
) (entities.BinanceTradingKey, error) {
	binanceTradingKey := entities.BinanceTradingKey{}

	result := binanceTradingKeyRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "user_id", Value: userID}).
		First(&binanceTradingKey)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured
	}
	if result.Error != nil {
		return entities.BinanceTradingKey{}, fmt.Errorf("find binance trading key: %w", result.Error)
	}

	return binanceTradingKey, nil
}

// Replace upserts on the unique user index, so concurrent saves cannot create two rows, and the row lock it takes waits out any switch being turned on against the old key.
func (binanceTradingKeyRepository *BinanceTradingKeyRepository) Replace(
	executionContext context.Context,
	binanceTradingKey entities.BinanceTradingKey,
	uncoveredBotMarketDataKinds []string,
) (entities.BinanceTradingKey, error) {
	transactionError := binanceTradingKeyRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			upsertResult := transaction.
				Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "user_id"}},
					DoUpdates: clause.AssignmentColumns([]string{
						"sealed_api_key", "sealed_secret_key", "api_key_tail",
						"spot_trading_enabled", "contract_trading_enabled", "updated_at",
					}),
				}).
				Create(&binanceTradingKey)
			if upsertResult.Error != nil {
				return fmt.Errorf("save binance trading key: %w", upsertResult.Error)
			}

			if len(uncoveredBotMarketDataKinds) == 0 {
				return nil
			}

			// The ORM's IN clause only takes untyped values.
			uncoveredKinds := make([]any, 0, len(uncoveredBotMarketDataKinds))
			for _, uncoveredBotMarketDataKind := range uncoveredBotMarketDataKinds {
				uncoveredKinds = append(uncoveredKinds, uncoveredBotMarketDataKind)
			}

			switchOffResult := transaction.Model(&entities.StrategyBot{}).
				Where(clause.Eq{Column: "owner_id", Value: binanceTradingKey.UserID}).
				Where(clause.IN{Column: "market_data_kind", Values: uncoveredKinds}).
				UpdateColumn("auto_order_enabled", false)
			if switchOffResult.Error != nil {
				return fmt.Errorf("switch off uncovered auto order: %w", switchOffResult.Error)
			}

			return nil
		})
	if transactionError != nil {
		return entities.BinanceTradingKey{}, transactionError
	}

	return binanceTradingKey, nil
}

// DeleteByUser switches every bot off before deleting, inside one transaction, so a failure leaves both untouched.
func (binanceTradingKeyRepository *BinanceTradingKeyRepository) DeleteByUser(
	executionContext context.Context, userID uint,
) error {
	return binanceTradingKeyRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			deleteResult := transaction.
				Where(clause.Eq{Column: "user_id", Value: userID}).
				Delete(&entities.BinanceTradingKey{})
			if deleteResult.Error != nil {
				return fmt.Errorf("delete binance trading key: %w", deleteResult.Error)
			}

			switchOffResult := transaction.Model(&entities.StrategyBot{}).
				Where(clause.Eq{Column: "owner_id", Value: userID}).
				UpdateColumn("auto_order_enabled", false)
			if switchOffResult.Error != nil {
				return fmt.Errorf("switch off auto order: %w", switchOffResult.Error)
			}

			return nil
		})
}
