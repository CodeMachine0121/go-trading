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

// PublishedStrategyRepository records which strategies are on the marketplace.
type PublishedStrategyRepository struct {
	database *gorm.DB
}

func NewPublishedStrategyRepository(database *gorm.DB) *PublishedStrategyRepository {
	return &PublishedStrategyRepository{database: database}
}

// Publish puts this strategy on the marketplace, or leaves it exactly where it is.
//
// The insert is told to do nothing on conflict rather than being preceded by a
// look, and that is what makes publishing twice one row: two requests arriving
// together both find the shelf empty if they look first, and only one of them
// survives the unique index if they do not. Doing nothing also preserves the
// original moment for free — the row that is already there is not touched.
func (publishedStrategyRepository *PublishedStrategyRepository) Publish(
	executionContext context.Context, strategyID uint, publishedAt time.Time,
) error {
	publication := entities.PublishedStrategy{StrategyID: strategyID, PublishedAt: publishedAt.UTC()}

	result := publishedStrategyRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "strategy_id"}}, DoNothing: true}).
		Create(&publication)
	if result.Error != nil {
		return fmt.Errorf("publish strategy: %w", result.Error)
	}

	return nil
}

// Withdraw takes this strategy off the marketplace. Every adoption of it goes too,
// and not because of a second statement here: the adoptions hang off this row with
// a cascade, so deleting it deletes them. Withdrawing something that is not there
// affects no rows and is not a failure — what was asked for already holds.
func (publishedStrategyRepository *PublishedStrategyRepository) Withdraw(
	executionContext context.Context, strategyID uint,
) error {
	result := publishedStrategyRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "strategy_id", Value: strategyID}).
		Delete(&entities.PublishedStrategy{})
	if result.Error != nil {
		return fmt.Errorf("withdraw strategy: %w", result.Error)
	}

	return nil
}

// FindOne returns this strategy's place on the marketplace, or
// ErrStrategyNotPublished when it has none.
func (publishedStrategyRepository *PublishedStrategyRepository) FindOne(
	executionContext context.Context, strategyID uint,
) (entities.PublishedStrategy, error) {
	publication := entities.PublishedStrategy{}

	result := publishedStrategyRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "strategy_id", Value: strategyID}).
		First(&publication)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.PublishedStrategy{}, domains.ErrStrategyNotPublished
	}
	if result.Error != nil {
		return entities.PublishedStrategy{}, fmt.Errorf("find published strategy: %w", result.Error)
	}

	return publication, nil
}
