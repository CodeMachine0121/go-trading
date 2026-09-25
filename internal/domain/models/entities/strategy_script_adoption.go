package entities

import "time"

// StrategyScriptAdoption is one person putting one published strategy script on their shelf; the unique pair makes adopting twice collide.
type StrategyScriptAdoption struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"not null;index:idx_strategy_adoptions_user;uniqueIndex:idx_strategy_adoptions_user_strategy"`
	// StrategyScriptID references the publication, so withdrawing it removes this row.
	StrategyScriptID uint      `gorm:"column:strategy_id;not null;uniqueIndex:idx_strategy_adoptions_user_strategy"`
	AdoptedAt        time.Time `gorm:"type:timestamptz;not null"`
}

// TableName keeps the pre-rename table name; see StrategyScript.TableName.
func (strategyScriptAdoption StrategyScriptAdoption) TableName() string {
	return "StrategyAdoptions"
}
