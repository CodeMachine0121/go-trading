package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// Strategy is one saved strategy: an algorithm, who it belongs to, and nothing else. How coarse the K
// candles are, how many of them, and up to when are all decided by whoever runs it,
// so the same algorithm can be run at any coarseness over any stretch of market.
// It is a plain data model: fields, persistence mapping and shape conversion only,
// no business rules.
//
// The name carries a unique index because a name is what a person recognises a
// strategy by, and the index — not a read-then-write check — is what actually makes
// it unique: two creates arriving at once both pass a check, and only one passes an
// index. It also gives renaming a strategy to its own current name for free, since
// the row it collides with is itself.
//
// That index spans the owner as well as the name, so it is uniqueness *within one
// person's collection*. A name is what its owner recognises a strategy by, and
// nobody recognises a stranger's; one shared pool of names would only mean the
// first person here takes "二十根均線" away from everybody else forever.
type Strategy struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is the person this strategy belongs to. It is settled when the
	// strategy is created and never changes: it is not on the list of columns a
	// rewrite may touch, so "a strategy cannot change hands" is something the write
	// path cannot express rather than something it remembers not to do.
	OwnerID uint   `gorm:"not null;index:idx_strategies_owner;uniqueIndex:idx_strategies_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategies_owner_name"`
	// Description is what the owner says this strategy is for. It may be empty, and
	// on the marketplace it is the only thing there is to judge by — the script is
	// never handed out, so a strategy with no description is a name and nothing else.
	Description string    `gorm:"type:text;not null;default:''"`
	Script      string    `gorm:"type:text;not null"`
	ResultType  string    `gorm:"size:32;not null"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`
	// Owner is declared so that a marketplace listing can name who published each
	// strategy without a second copy of that fact living on the publication row.
	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// Publication is "this strategy is on the marketplace", and it is declared here
	// with a cascade so that deleting a strategy takes it off the marketplace —
	// and, through the publication's own cascade, out of everybody's shelf. That
	// chain is the whole implementation of those two rules; no Go code performs it,
	// which is why no Go code can forget to.
	Publication *PublishedStrategy `gorm:"foreignKey:StrategyID;constraint:OnDelete:CASCADE"`
	// Parameters belong to this strategy and to nothing else: they are never read,
	// created or deleted on their own, which is why they have no repository of their
	// own and travel with the strategy that owns them.
	Parameters []StrategyParameter `gorm:"foreignKey:StrategyID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to Strategies instead of GORM's default strategies.
func (strategy Strategy) TableName() string {
	return "Strategies"
}

// ToDto converts this record into the shape the domain hands outwards. Both times
// are always handed out in universal time, whatever zone they were read back in.
func (strategy Strategy) ToDto() dto.StrategyDto {
	return dto.StrategyDto{
		ID:          strategy.ID,
		Name:        strategy.Name,
		Description: strategy.Description,
		Script:      strategy.Script,
		ResultType:  strategy.ResultType,
		CreatedAt:   strategy.CreatedAt.UTC(),
		UpdatedAt:   strategy.UpdatedAt.UTC(),
		Parameters:  strategy.parameterDtos(),
	}
}

// parameterDtos hands out this strategy's knobs, always as a list rather than
// sometimes nothing: a strategy with no knobs has an empty list, not an absence.
func (strategy Strategy) parameterDtos() []dto.StrategyParameterDto {
	parameterDtos := make([]dto.StrategyParameterDto, 0, len(strategy.Parameters))
	for _, parameter := range strategy.Parameters {
		parameterDtos = append(parameterDtos, parameter.ToDto())
	}

	return parameterDtos
}

// ToParameterDtos hands out this strategy's knobs to whoever is assembling a shape
// other than StrategyDto — the marketplace listing shows the same knobs without the
// script. It exists so that "the knobs" has one spelling; ToDto goes through the
// same helper.
func (strategy Strategy) ToParameterDtos() []dto.StrategyParameterDto {
	return strategy.parameterDtos()
}
