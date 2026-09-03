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

// StrategyNameIndex is the unique index on a strategy's name, and
// uniqueViolationCode is what PostgreSQL calls a broken unique constraint. Together
// they are how a name clash is told apart from any other constraint on the table —
// the primary key included, which breaks when a restored dump leaves the identifier
// sequence behind and has nothing to do with anyone's choice of name.
//
// The index name is repeated from the entity's tag because a struct tag cannot hold
// a constant. If the two ever drift, storing a duplicate name stops being reported
// as a conflict and starts being reported as a storage failure. It is exported so
// that the agreement between the two spellings is asserted by a test needing no
// database, rather than only by one that skips when there is none.
const StrategyNameIndex = "idx_strategies_name"

// uniqueViolationCode is what PostgreSQL calls a broken unique constraint.
const uniqueViolationCode = "23505"

// strategyWritableColumns are the only columns a rewrite may touch. Naming them is
// what makes "the identifier and the time it was first saved never change" true:
// they are not on the list, so no update can reach them however the entity handed in
// was filled.
var strategyWritableColumns = []string{
	"name", "script", "result_type", "aggregation_interval", "candle_count",
}

// StrategyRepository stores saved strategies in PostgreSQL.
type StrategyRepository struct {
	database *gorm.DB
}

func NewStrategyRepository(database *gorm.DB) *StrategyRepository {
	return &StrategyRepository{database: database}
}

// Save stores a new strategy, letting the unique index on the name decide whether it
// may exist. Asking first and creating afterwards would let two requests arriving at
// once both find the name free.
func (strategyRepository *StrategyRepository) Save(
	executionContext context.Context, strategy entities.Strategy,
) (entities.Strategy, error) {
	result := strategyRepository.database.WithContext(executionContext).Create(&strategy)
	if writeError := strategyRepository.writeFailureOf(result.Error, strategy.Name, "save"); writeError != nil {
		return entities.Strategy{}, writeError
	}

	return strategy, nil
}

// Update rewrites the five things a strategy remembers and hands back the strategy as
// it now stands. Only the writable columns are sent, so the identifier and the time
// the strategy was first saved are out of reach by construction.
func (strategyRepository *StrategyRepository) Update(
	executionContext context.Context, strategy entities.Strategy,
) (entities.Strategy, error) {
	// The write and the read-back share one transaction, so what comes back is what
	// this call stored. Apart, a second rewrite landing between them would hand this
	// caller somebody else's values as though they were its own — and a deletion
	// landing there would report not found for a row this call had just written.
	updatedStrategy := entities.Strategy{}

	transactionError := strategyRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The strategy handed to Model carries the identifier, which is what
			// picks the row; only the writable columns are then sent.
			result := transaction.
				Model(&entities.Strategy{ID: strategy.ID}).
				Select(strategyWritableColumns).
				Updates(strategy)

			if writeError := strategyRepository.writeFailureOf(
				result.Error, strategy.Name, "update"); writeError != nil {
				return writeError
			}

			// Reading it back is also what reports a strategy that is not there:
			// nothing was rewritten, so nothing can be found. Checking the rows
			// written first would ask the same question twice.
			readBack := transaction.First(&updatedStrategy, strategy.ID)
			if errors.Is(readBack.Error, gorm.ErrRecordNotFound) {
				return domains.StrategyNotFound(strategy.ID)
			}
			if readBack.Error != nil {
				return fmt.Errorf("find strategy: %w", readBack.Error)
			}

			return nil
		})

	if transactionError != nil {
		return entities.Strategy{}, transactionError
	}

	return updatedStrategy, nil
}

// writeFailureOf says what went wrong with a write, in the terms the domain uses. A
// broken name index is the one storage failure that is really a business answer: the
// name belongs to another strategy. It is written here once because both writes
// reach the same index and owe the caller the same answer.
//
// Every other broken constraint stays a storage failure. Answering "that name is
// taken" for a clash the name had no part in would send whoever reads it hunting for
// a strategy that does not exist.
func (strategyRepository *StrategyRepository) writeFailureOf(
	writeError error, name string, attempt string,
) error {
	if strategyRepository.isNameAlreadyHeld(writeError) {
		return fmt.Errorf("%w: 策略名稱「%s」已被使用", domains.ErrStrategyNameConflict, name)
	}
	if writeError != nil {
		return fmt.Errorf("%s strategy: %w", attempt, writeError)
	}

	return nil
}

// isNameAlreadyHeld says whether this write broke the name index specifically.
func (strategyRepository *StrategyRepository) isNameAlreadyHeld(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == StrategyNameIndex
}

// FindOne returns the strategy carrying this identifier.
func (strategyRepository *StrategyRepository) FindOne(executionContext context.Context, id uint) (entities.Strategy, error) {
	strategy := entities.Strategy{}

	result := strategyRepository.database.WithContext(executionContext).First(&strategy, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.Strategy{}, domains.StrategyNotFound(id)
	}
	if result.Error != nil {
		return entities.Strategy{}, fmt.Errorf("find strategy: %w", result.Error)
	}

	return strategy, nil
}

// FindAll returns every saved strategy, ordered by name.
func (strategyRepository *StrategyRepository) FindAll(executionContext context.Context) ([]entities.Strategy, error) {
	strategies := make([]entities.Strategy, 0)

	result := strategyRepository.database.WithContext(executionContext).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "name"}}).
		Find(&strategies)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategies: %w", result.Error)
	}

	return strategies, nil
}

// Delete removes the strategy for good. There is no keeping of what was deleted:
// a name that is still held by something nobody can read is a name nobody can
// explain, and this is a single person's own collection.
func (strategyRepository *StrategyRepository) Delete(executionContext context.Context, id uint) error {
	result := strategyRepository.database.WithContext(executionContext).Delete(&entities.Strategy{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete strategy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.StrategyNotFound(id)
	}

	return nil
}
