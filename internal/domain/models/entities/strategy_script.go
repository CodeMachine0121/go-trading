package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyScript is one saved algorithm and its owner; names are unique per owner via an index, not a read-then-write check.
type StrategyScript struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is never on the rewrite column list, so a script cannot change hands.
	OwnerID uint   `gorm:"not null;index:idx_strategies_owner;uniqueIndex:idx_strategies_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategies_owner_name"`
	// Description is the only thing marketplace users can judge by, since the script itself is never handed out.
	Description string `gorm:"type:text;not null;default:''"`
	Script      string `gorm:"type:text;not null"`
	ResultType  string `gorm:"size:32;not null"`
	// MarketDataKind is fixed at creation; the default matches scripts saved before the choice existed.
	MarketDataKind string    `gorm:"size:32;not null;default:'kCandle'"`
	CreatedAt      time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt      time.Time `gorm:"type:timestamptz;not null"`
	Owner          User      `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// Publication cascades so deleting a script removes it from the marketplace and, transitively, from every shelf.
	Publication *PublishedStrategyScript  `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
	Parameters  []StrategyScriptParameter `gorm:"foreignKey:StrategyScriptID;constraint:OnDelete:CASCADE"`
}

// TableName keeps the pre-rename "Strategies" table because AutoMigrate would create a new table rather than move the data.
func (strategyScript StrategyScript) TableName() string {
	return "Strategies"
}

func (strategyScript StrategyScript) ToDto() dto.StrategyScriptDto {
	return dto.StrategyScriptDto{
		ID:             strategyScript.ID,
		Name:           strategyScript.Name,
		Description:    strategyScript.Description,
		Script:         strategyScript.Script,
		ResultType:     strategyScript.ResultType,
		MarketDataKind: strategyScript.MarketDataKind,
		Published:      strategyScript.Publication != nil,
		CreatedAt:      strategyScript.CreatedAt.UTC(),
		UpdatedAt:      strategyScript.UpdatedAt.UTC(),
		Parameters:     strategyScript.parameterDtos(),
	}
}

// parameterDtos always returns a non-nil list.
func (strategyScript StrategyScript) parameterDtos() []dto.StrategyScriptParameterDto {
	parameterDtos := make([]dto.StrategyScriptParameterDto, 0, len(strategyScript.Parameters))
	for _, parameter := range strategyScript.Parameters {
		parameterDtos = append(parameterDtos, parameter.ToDto())
	}

	return parameterDtos
}
