package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// Strategy is one saved strategy: an algorithm and nothing else. How coarse the K
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
type Strategy struct {
	ID                  uint      `gorm:"primaryKey"`
	Name                string    `gorm:"size:128;not null;uniqueIndex:idx_strategies_name"`
	Script              string    `gorm:"type:text;not null"`
	ResultType          string    `gorm:"size:32;not null"`
	CreatedAt           time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt           time.Time `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to Strategies instead of GORM's default strategies.
func (strategy Strategy) TableName() string {
	return "Strategies"
}

// ToDto converts this record into the shape the domain hands outwards. Both times
// are always handed out in universal time, whatever zone they were read back in.
func (strategy Strategy) ToDto() dto.StrategyDto {
	return dto.StrategyDto{
		ID:                  strategy.ID,
		Name:                strategy.Name,
		Script:              strategy.Script,
		ResultType:          strategy.ResultType,
		CreatedAt:           strategy.CreatedAt.UTC(),
		UpdatedAt:           strategy.UpdatedAt.UTC(),
	}
}
