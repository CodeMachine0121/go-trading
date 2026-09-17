package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyScript is one saved strategy script: an algorithm, who it belongs to, and nothing else. How coarse the K
// candles are, how many of them, and up to when are all decided by whoever runs it,
// so the same algorithm can be run at any coarseness over any stretch of market.
// It is a plain data model: fields, persistence mapping and shape conversion only,
// no business rules.
//
// The name carries a unique index because a name is what a person recognises a
// strategy script by, and the index — not a read-then-write check — is what actually makes
// it unique: two creates arriving at once both pass a check, and only one passes an
// index. It also gives renaming a strategy script to its own current name for free, since
// the row it collides with is itself.
//
// That index spans the owner as well as the name, so it is uniqueness *within one
// person's collection*. A name is what its owner recognises a strategy script by, and
// nobody recognises a stranger's; one shared pool of names would only mean the
// first person here takes "二十根均線" away from everybody else forever.
type StrategyScript struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is the person this strategy script belongs to. It is settled when the
	// strategy script is created and never changes: it is not on the list of columns a
	// rewrite may touch, so "a strategy script cannot change hands" is something the write
	// path cannot express rather than something it remembers not to do.
	OwnerID uint   `gorm:"not null;index:idx_strategies_owner;uniqueIndex:idx_strategies_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategies_owner_name"`
	// Description is what the owner says this strategy script is for. It may be empty, and
	// on the marketplace it is the only thing there is to judge by — the script is
	// never handed out, so a strategy script with no description is a name and nothing else.
	Description string    `gorm:"type:text;not null;default:''"`
	Script      string    `gorm:"type:text;not null"`
	ResultType  string    `gorm:"size:32;not null"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`
	// Owner is declared so that a marketplace listing can name who published each
	// strategy script without a second copy of that fact living on the publication row.
	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// Publication is "this strategy script is on the marketplace", and it is declared here
	// with a cascade so that deleting a strategy script takes it off the marketplace —
	// and, through the publication's own cascade, out of everybody's shelf. That
	// chain is the whole implementation of those two rules; no Go code performs it,
	// which is why no Go code can forget to.
	Publication *PublishedStrategyScript `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
	// Parameters belong to this strategy script and to nothing else: they are never read,
	// created or deleted on their own, which is why they have no repository of their
	// own and travel with the strategy script that owns them.
	Parameters []StrategyScriptParameter `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to Strategies. The table predates the rename of this
// model from Strategy to StrategyScript and keeps its old name on purpose:
// AutoMigrate creates a new table rather than moving one, so renaming it here
// would leave every saved strategy script behind in a table nothing reads.
func (strategyScript StrategyScript) TableName() string {
	return "Strategies"
}

// ToDto converts this record into the shape the domain hands outwards. Both times
// are always handed out in universal time, whatever zone they were read back in.
func (strategyScript StrategyScript) ToDto() dto.StrategyScriptDto {
	return dto.StrategyScriptDto{
		ID:          strategyScript.ID,
		Name:        strategyScript.Name,
		Description: strategyScript.Description,
		Script:      strategyScript.Script,
		ResultType:  strategyScript.ResultType,
		// A publication read back with the strategy script is what says it is out there.
		// Asking the marketplace separately would be one more query per strategy script,
		// and the association is already declared right here.
		Published:  strategyScript.Publication != nil,
		CreatedAt:  strategyScript.CreatedAt.UTC(),
		UpdatedAt:  strategyScript.UpdatedAt.UTC(),
		Parameters: strategyScript.parameterDtos(),
	}
}

// parameterDtos hands out this strategy script's knobs, always as a list rather than
// sometimes nothing: a strategy script with no knobs has an empty list, not an absence.
func (strategyScript StrategyScript) parameterDtos() []dto.StrategyScriptParameterDto {
	parameterDtos := make([]dto.StrategyScriptParameterDto, 0, len(strategyScript.Parameters))
	for _, parameter := range strategyScript.Parameters {
		parameterDtos = append(parameterDtos, parameter.ToDto())
	}

	return parameterDtos
}
