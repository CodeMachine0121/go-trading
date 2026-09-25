package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// foreignKeyViolationCode is PostgreSQL's foreign-key violation; an adoption has exactly one foreign key (asserted by a schema test), so the code alone identifies the failure.
const foreignKeyViolationCode = "23503"

type StrategyScriptAdoptionRepository struct {
	database *gorm.DB
}

func NewStrategyScriptAdoptionRepository(database *gorm.DB) *StrategyScriptAdoptionRepository {
	return &StrategyScriptAdoptionRepository{database: database}
}

// Adopt is idempotent via the unique index; adopting an unpublished script breaks the foreign key and is reported as not found.
func (strategyScriptAdoptionRepository *StrategyScriptAdoptionRepository) Adopt(
	executionContext context.Context, userID uint, strategyScriptID uint, adoptedAt time.Time,
) error {
	adoption := entities.StrategyScriptAdoption{UserID: userID, StrategyScriptID: strategyScriptID, AdoptedAt: adoptedAt.UTC()}

	result := strategyScriptAdoptionRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "strategy_id"}},
			DoNothing: true,
		}).
		Create(&adoption)
	if strategyScriptAdoptionRepository.isNotOnTheMarketplace(result.Error) {
		return domains.StrategyScriptNotFound(strategyScriptID)
	}
	if result.Error != nil {
		return fmt.Errorf("adopt strategy script: %w", result.Error)
	}

	return nil
}

// Abandon on something never adopted affects no rows and is not a failure.
func (strategyScriptAdoptionRepository *StrategyScriptAdoptionRepository) Abandon(
	executionContext context.Context, userID uint, strategyScriptID uint,
) error {
	result := strategyScriptAdoptionRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "user_id", Value: userID}).
		Where(clause.Eq{Column: "strategy_id", Value: strategyScriptID}).
		Delete(&entities.StrategyScriptAdoption{})
	if result.Error != nil {
		return fmt.Errorf("abandon strategy script: %w", result.Error)
	}

	return nil
}

// isNotOnTheMarketplace reports a missing publication; every other storage failure stays a storage failure.
func (strategyScriptAdoptionRepository *StrategyScriptAdoptionRepository) isNotOnTheMarketplace(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == foreignKeyViolationCode
}
