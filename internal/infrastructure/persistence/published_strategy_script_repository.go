package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PublishedStrategyScriptRepository struct {
	database *gorm.DB
}

func NewPublishedStrategyScriptRepository(database *gorm.DB) *PublishedStrategyScriptRepository {
	return &PublishedStrategyScriptRepository{database: database}
}

// Publish inserts with ON CONFLICT DO NOTHING, so concurrent publishes yield one row and the original publish time is preserved.
func (publishedStrategyScriptRepository *PublishedStrategyScriptRepository) Publish(
	executionContext context.Context, strategyScriptID uint, publishedAt time.Time,
) error {
	publication := entities.PublishedStrategyScript{StrategyScriptID: strategyScriptID, PublishedAt: publishedAt.UTC()}

	result := publishedStrategyScriptRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "strategy_id"}}, DoNothing: true}).
		Create(&publication)
	if result.Error != nil {
		return fmt.Errorf("publish strategy script: %w", result.Error)
	}

	return nil
}

// Withdraw deletes the listing, cascading to its adoptions; withdrawing an unpublished script is not an error.
func (publishedStrategyScriptRepository *PublishedStrategyScriptRepository) Withdraw(
	executionContext context.Context, strategyScriptID uint,
) error {
	result := publishedStrategyScriptRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "strategy_id", Value: strategyScriptID}).
		Delete(&entities.PublishedStrategyScript{})
	if result.Error != nil {
		return fmt.Errorf("withdraw strategy script: %w", result.Error)
	}

	return nil
}

func (publishedStrategyScriptRepository *PublishedStrategyScriptRepository) FindOne(
	executionContext context.Context, strategyScriptID uint,
) (entities.PublishedStrategyScript, error) {
	publication := entities.PublishedStrategyScript{}

	result := publishedStrategyScriptRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "strategy_id", Value: strategyScriptID}).
		First(&publication)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished
	}
	if result.Error != nil {
		return entities.PublishedStrategyScript{}, fmt.Errorf("find published strategy script: %w", result.Error)
	}

	return publication, nil
}
