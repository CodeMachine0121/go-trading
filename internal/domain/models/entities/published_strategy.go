package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// PublishedStrategy is the fact that a strategy is on the marketplace. It holds
// nothing about the strategy itself — only that it is out there, and since when.
//
// It carries no publisher of its own on purpose. Only an owner can publish, and a
// strategy never changes hands, so a publisher column could only ever agree with
// the strategy's owner or drift from it — never inform anyone. Who published a
// strategy is read from the strategy, one hop away. (The day strategies can be
// transferred, "who published it back then" becomes a fact worth its own column;
// today it is a copy.)
//
// StrategyID is unique, which is what makes publishing twice one row rather than a
// check somebody has to remember: the second publish collides with the first, and
// the first publish's moment is the one that survives.
//
// It is a plain data model: fields, persistence mapping and shape conversion only.
type PublishedStrategy struct {
	ID         uint `gorm:"primaryKey"`
	StrategyID uint `gorm:"not null;uniqueIndex:idx_published_strategies_strategy"`
	// PublishedAt is when this strategy first reached the marketplace. Publishing an
	// already-published strategy leaves it alone — the strategy has been out there
	// since then, and pressing the button again does not change when that started.
	PublishedAt time.Time `gorm:"type:timestamptz;not null"`
	// Strategy is declared so a marketplace listing reads the strategy and its owner
	// in one go rather than one query per row.
	Strategy Strategy `gorm:"foreignKey:StrategyID;constraint:OnDelete:CASCADE"`
	// Adoptions belong to this publication and to nothing else. The cascade is the
	// entire implementation of "taking a strategy off the marketplace clears it from
	// everybody's shelf": deleting this row deletes them, so there is no line of Go
	// that could be forgotten, and no window where a shelf points at a strategy the
	// owner has already withdrawn.
	Adoptions []StrategyAdoption `gorm:"foreignKey:StrategyID;references:StrategyID;constraint:OnDelete:CASCADE"`
}

// ToDto is this publication as everybody but the owner sees it: the strategy's
// name, what it is for, what it produces, who put it there and when — and no
// algorithm, because the shape it converts into has nowhere to put one.
//
// It asks no permission, and needs none: a published strategy is public, and
// nothing here is not.
func (publishedStrategy PublishedStrategy) ToDto() dto.PublishedStrategyDto {
	return dto.PublishedStrategyDto{
		ID:             publishedStrategy.Strategy.ID,
		Name:           publishedStrategy.Strategy.Name,
		Description:    publishedStrategy.Strategy.Description,
		ResultType:     publishedStrategy.Strategy.ResultType,
		PublisherEmail: publishedStrategy.Strategy.Owner.Email,
		PublishedAt:    publishedStrategy.PublishedAt.UTC(),
		Parameters:     publishedStrategy.Strategy.parameterDtos(),
	}
}

// TableName pins the table to PublishedStrategies instead of GORM's default.
func (publishedStrategy PublishedStrategy) TableName() string {
	return "PublishedStrategies"
}
