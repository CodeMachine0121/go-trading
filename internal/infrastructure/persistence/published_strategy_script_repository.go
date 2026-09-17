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

// PublishedStrategyScriptRepository records which strategy scripts are on the marketplace.
type PublishedStrategyScriptRepository struct {
	database *gorm.DB
}

func NewPublishedStrategyScriptRepository(database *gorm.DB) *PublishedStrategyScriptRepository {
	return &PublishedStrategyScriptRepository{database: database}
}

// Publish puts this strategy script on the marketplace, or leaves it exactly where it is.
//
// The insert is told to do nothing on conflict rather than being preceded by a
// look, and that is what makes publishing twice one row: two requests arriving
// together both find the shelf empty if they look first, and only one of them
// survives the unique index if they do not. Doing nothing also preserves the
// original moment for free — the row that is already there is not touched.
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

// Withdraw takes this strategy script off the marketplace. Every adoption of it goes too,
// and not because of a second statement here: the adoptions hang off this row with
// a cascade, so deleting it deletes them. Withdrawing something that is not there
// affects no rows and is not a failure — what was asked for already holds.
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

// FindOne returns this strategy script's place on the marketplace, or
// ErrStrategyScriptNotPublished when it has none.
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
