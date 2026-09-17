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

// TradingStrategyRepository stores trading strategies, their signal sources and
// their two condition trees, in PostgreSQL.
type TradingStrategyRepository struct {
	database *gorm.DB
}

func NewTradingStrategyRepository(database *gorm.DB) *TradingStrategyRepository {
	return &TradingStrategyRepository{database: database}
}

// Save stores this trading strategy whole, replacing whatever it had before.
//
// Everything happens in one transaction, because a set of rules whose conditions
// were replaced but whose sources were not is a set of rules that can name a label
// that no longer exists — and every bot following it would then run that way, every
// few minutes.
//
// The children are cleared and written again rather than compared and patched. A
// condition tree holds at most thirty-two nodes and is only ever read and written
// whole; the reads a diff would save are worth less than the ways a diff can be
// wrong.
func (tradingStrategyRepository *TradingStrategyRepository) Save(
	executionContext context.Context, tradingStrategy entities.TradingStrategy,
) (entities.TradingStrategy, error) {
	savedTradingStrategy := tradingStrategy

	transactionError := tradingStrategyRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The row goes first and alone: its identifier is what every child row
			// needs, and on a create nobody knows it until this returns. Omitting
			// the associations is what stops GORM writing them here in a shape this
			// code would then have to undo.
			tradingStrategyRow := tradingStrategy
			tradingStrategyRow.SignalSources = nil
			tradingStrategyRow.ConditionNodes = nil

			if tradingStrategyRow.ID == 0 {
				if createError := transaction.Omit(clause.Associations).
					Create(&tradingStrategyRow).Error; createError != nil {
					return createError
				}
			} else {
				// Naming the column makes an empty value mean empty rather than
				// "unchanged", which is how GORM reads a struct otherwise — and it
				// is what keeps a rewrite away from the owner and the created time.
				updates := transaction.Model(&entities.TradingStrategy{}).
					Where(clause.Eq{Column: "id", Value: tradingStrategyRow.ID}).
					Select("name").
					Updates(entities.TradingStrategy{Name: tradingStrategyRow.Name})
				if updates.Error != nil {
					return updates.Error
				}

				if deleteError := transaction.
					Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyRow.ID}).
					Delete(&entities.TradingStrategySignalSource{}).Error; deleteError != nil {
					return deleteError
				}

				// Deleting the roots takes their descendants with them through the
				// node table's own cascade, so this does not walk the tree — and
				// therefore cannot walk it wrong.
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

// writeConditionSubtree writes one node and everything under it, handing each child
// the identifier its parent has just been given.
//
// It descends explicitly rather than relying on the store to write a nested
// association for it. One level of nesting is something an ORM will do; five is
// something to find out about at three in the morning.
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

// TradingStrategyNameIndex is the index that makes a name unique within its owner's
// collection. It is named here because the write path has to recognise this one
// breaking specifically — any other broken constraint is a fault, not a person
// reusing a name.
const TradingStrategyNameIndex = "idx_trading_strategies_owner_name"

// writeFailureOf turns a failed write into the refusal it actually is. A broken name
// index is a person reusing a name they already have; anything else is a fault, and
// dressing it up as a name conflict would send them off renaming something that was
// never the problem.
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

// FindOne returns it with its sources and both trees.
func (tradingStrategyRepository *TradingStrategyRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.TradingStrategy, error) {
	tradingStrategy := entities.TradingStrategy{}

	// The condition is spelled out rather than given as a struct, because GORM
	// drops zero-valued struct fields — and an identifier of nothing would become
	// no condition at all, handing back whichever row happens to be first.
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

// FindAllByOwner returns this person's trading strategies, by name.
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

// Delete removes it. Its sources and condition nodes go with it by cascade.
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
