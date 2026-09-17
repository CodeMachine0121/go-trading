package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// PublishedStrategyScript is the fact that a strategy script is on the marketplace. It holds
// nothing about the strategy script itself — only that it is out there, and since when.
//
// It carries no publisher of its own on purpose. Only an owner can publish, and a
// strategy script never changes hands, so a publisher column could only ever agree with
// the strategy script's owner or drift from it — never inform anyone. Who published a
// strategy script is read from the strategy script, one hop away. (The day strategy scripts can be
// transferred, "who published it back then" becomes a fact worth its own column;
// today it is a copy.)
//
// StrategyScriptID is unique, which is what makes publishing twice one row rather than a
// check somebody has to remember: the second publish collides with the first, and
// the first publish's moment is the one that survives.
//
// It is a plain data model: fields, persistence mapping and shape conversion only.
type PublishedStrategyScript struct {
	ID               uint `gorm:"primaryKey"`
	StrategyScriptID uint `gorm:"column:strategy_id;not null;uniqueIndex:idx_published_strategies_strategy"`
	// PublishedAt is when this strategy script first reached the marketplace. Publishing an
	// already-published strategy script leaves it alone — the strategy script has been out there
	// since then, and pressing the button again does not change when that started.
	PublishedAt time.Time `gorm:"type:timestamptz;not null"`
	// StrategyScript is declared so a marketplace listing reads the strategy script and its owner
	// in one go rather than one query per row.
	StrategyScript StrategyScript `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
	// Adoptions belong to this publication and to nothing else. The cascade is the
	// entire implementation of "taking a strategy script off the marketplace clears it from
	// everybody's shelf": deleting this row deletes them, so there is no line of Go
	// that could be forgotten, and no window where a shelf points at a strategy script the
	// owner has already withdrawn.
	Adoptions []StrategyScriptAdoption `gorm:"foreignKey:StrategyScriptID;references:StrategyScriptID;constraint:OnDelete:CASCADE"`
}

// ToDto is this publication as everybody but the owner sees it: the strategy script's
// name, what it is for, what it produces, who put it there and when — and no
// algorithm, because the shape it converts into has nowhere to put one.
//
// It asks no permission, and needs none: a published strategy script is public, and
// nothing here is not.
func (publishedStrategyScript PublishedStrategyScript) ToDto() dto.PublishedStrategyScriptDto {
	return dto.PublishedStrategyScriptDto{
		ID:             publishedStrategyScript.StrategyScript.ID,
		Name:           publishedStrategyScript.StrategyScript.Name,
		Description:    publishedStrategyScript.StrategyScript.Description,
		ResultType:     publishedStrategyScript.StrategyScript.ResultType,
		PublisherEmail: publishedStrategyScript.StrategyScript.Owner.Email,
		PublishedAt:    publishedStrategyScript.PublishedAt.UTC(),
		Parameters:     publishedStrategyScript.StrategyScript.parameterDtos(),
	}
}

// TableName pins the table to PublishedStrategies, the name it had before this
// model was renamed. See StrategyScript.TableName for why the old name stays.
func (publishedStrategyScript PublishedStrategyScript) TableName() string {
	return "PublishedStrategies"
}
