package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// PublishedStrategyScript marks a strategy script as on the marketplace; the unique StrategyScriptID makes a repeat publish collide and keeps the first PublishedAt.
type PublishedStrategyScript struct {
	ID               uint `gorm:"primaryKey"`
	StrategyScriptID uint `gorm:"column:strategy_id;not null;uniqueIndex:idx_published_strategies_strategy"`
	// PublishedAt is when the script first reached the marketplace; republishing does not change it.
	PublishedAt    time.Time      `gorm:"type:timestamptz;not null"`
	StrategyScript StrategyScript `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
}

// ToDto is the public view of this publication; it carries no algorithm.
func (publishedStrategyScript PublishedStrategyScript) ToDto() dto.PublishedStrategyScriptDto {
	return dto.PublishedStrategyScriptDto{
		ID:             publishedStrategyScript.StrategyScript.ID,
		Name:           publishedStrategyScript.StrategyScript.Name,
		Description:    publishedStrategyScript.StrategyScript.Description,
		ResultType:     publishedStrategyScript.StrategyScript.ResultType,
		MarketDataKind: publishedStrategyScript.StrategyScript.MarketDataKind,
		PublisherEmail: publishedStrategyScript.StrategyScript.Owner.Email,
		PublishedAt:    publishedStrategyScript.PublishedAt.UTC(),
		Parameters:     publishedStrategyScript.StrategyScript.parameterDtos(),
	}
}

// TableName keeps the pre-rename table name; see StrategyScript.TableName.
func (publishedStrategyScript PublishedStrategyScript) TableName() string {
	return "PublishedStrategies"
}
