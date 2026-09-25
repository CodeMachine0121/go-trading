package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradingStrategyRepository struct {
	database *gorm.DB
}

func NewTradingStrategyRepository(database *gorm.DB) *TradingStrategyRepository {
	return &TradingStrategyRepository{database: database}
}

// Save replaces the strategy and all its children in one transaction, clearing and rewriting children rather than diffing.
func (tradingStrategyRepository *TradingStrategyRepository) Save(
	executionContext context.Context, tradingStrategy entities.TradingStrategy,
) (entities.TradingStrategy, error) {
	savedTradingStrategy := tradingStrategy

	transactionError := tradingStrategyRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The row is written first without associations because children need its identifier.
			tradingStrategyRow := tradingStrategy
			tradingStrategyRow.SignalSources = nil
			tradingStrategyRow.ConditionNodes = nil

			if tradingStrategyRow.ID == 0 {
				if createError := transaction.Omit(clause.Associations).
					Create(&tradingStrategyRow).Error; createError != nil {
					return createError
				}
			} else {
				// Named columns make empty values write through and keep owner, creation time and market data kind immutable.
				updates := transaction.Model(&entities.TradingStrategy{}).
					Where(clause.Eq{Column: "id", Value: tradingStrategyRow.ID}).
					Select("name", "contract_trading_mode").
					Updates(entities.TradingStrategy{
						Name:        tradingStrategyRow.Name,
						TradingMode: tradingStrategyRow.TradingMode,
					})
				if updates.Error != nil {
					return updates.Error
				}

				if deleteError := transaction.
					Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyRow.ID}).
					Delete(&entities.TradingStrategySignalSource{}).Error; deleteError != nil {
					return deleteError
				}

				// Deleting the roots cascades to their descendants.
				if deleteError := transaction.
					Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyRow.ID}).
					Delete(&entities.TradingStrategyConditionNode{}).Error; deleteError != nil {
					return deleteError
				}
			}

			for index := range tradingStrategy.SignalSources {
				signalSource := tradingStrategy.SignalSources[index]
				signalSource.ID = 0
				signalSource.TradingStrategyID = tradingStrategyRow.ID
				for valueIndex := range signalSource.ParameterValues {
					signalSource.ParameterValues[valueIndex].ID = 0
					signalSource.ParameterValues[valueIndex].TradingStrategySignalSourceID = 0
				}

				if createError := transaction.Create(&signalSource).Error; createError != nil {
					return createError
				}
			}

			for index := range tradingStrategy.ConditionNodes {
				if writeError := tradingStrategyRepository.writeConditionSubtree(
					transaction, tradingStrategyRow.ID, nil,
					tradingStrategy.ConditionNodes[index]); writeError != nil {
					return writeError
				}
			}

			savedTradingStrategy = tradingStrategyRow

			return nil
		})
	if transactionError != nil {
		return entities.TradingStrategy{}, tradingStrategyRepository.writeFailureOf(
			transactionError, tradingStrategy.Name)
	}

	return tradingStrategyRepository.FindOne(executionContext, savedTradingStrategy.ID)
}

// writeConditionSubtree writes nodes recursively, giving each child its parent's new identifier, rather than trusting ORM nested association writes.
func (tradingStrategyRepository *TradingStrategyRepository) writeConditionSubtree(
	transaction *gorm.DB, tradingStrategyID uint, parentID *uint,
	node entities.TradingStrategyConditionNode,
) error {
	children := node.Children

	nodeRow := node
	nodeRow.ID = 0
	nodeRow.TradingStrategyID = tradingStrategyID
	nodeRow.ParentID = parentID
	nodeRow.Children = nil
	nodeRow.Parent = nil

	if createError := transaction.Omit(clause.Associations).Create(&nodeRow).Error; createError != nil {
		return createError
	}

	for index := range children {
		if writeError := tradingStrategyRepository.writeConditionSubtree(
			transaction, tradingStrategyID, &nodeRow.ID, children[index]); writeError != nil {
			return writeError
		}
	}

	return nil
}

const TradingStrategyNameIndex = "idx_trading_strategies_owner_name"

// writeFailureOf maps a broken name index to a name conflict; anything else stays a fault.
func (tradingStrategyRepository *TradingStrategyRepository) writeFailureOf(
	writeError error, name string,
) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == TradingStrategyNameIndex {
		return fmt.Errorf(
			"%w: 交易策略名稱「%s」已被使用", domains.ErrTradingStrategyNameConflict, name)
	}

	return fmt.Errorf("save trading strategy: %w", writeError)
}

func (tradingStrategyRepository *TradingStrategyRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.TradingStrategy, error) {
	tradingStrategy := entities.TradingStrategy{}

	// A string condition is used because GORM drops zero-valued struct fields, which would match any row.
	result := tradingStrategyRepository.database.WithContext(executionContext).
		Preload("SignalSources.ParameterValues").
		Preload("SignalSources").
		Preload("ConditionNodes").
		Where(clause.Eq{Column: "id", Value: id}).
		First(&tradingStrategy)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.TradingStrategy{}, domains.TradingStrategyNotFound(id)
	}
	if result.Error != nil {
		return entities.TradingStrategy{}, fmt.Errorf("find trading strategy: %w", result.Error)
	}

	return tradingStrategy, nil
}

func (tradingStrategyRepository *TradingStrategyRepository) FindAllByOwner(
	executionContext context.Context, ownerID uint,
) ([]entities.TradingStrategy, error) {
	tradingStrategies := []entities.TradingStrategy{}

	result := tradingStrategyRepository.database.WithContext(executionContext).
		Preload("SignalSources.ParameterValues").
		Preload("SignalSources").
		Preload("ConditionNodes").
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("name ASC").
		Find(&tradingStrategies)
	if result.Error != nil {
		return nil, fmt.Errorf("list trading strategies: %w", result.Error)
	}

	return tradingStrategies, nil
}

// Delete removes the strategy; its sources and condition nodes cascade.
func (tradingStrategyRepository *TradingStrategyRepository) Delete(
	executionContext context.Context, id uint,
) error {
	result := tradingStrategyRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Delete(&entities.TradingStrategy{})
	if result.Error != nil {
		return fmt.Errorf("delete trading strategy: %w", result.Error)
	}

	return nil
}
