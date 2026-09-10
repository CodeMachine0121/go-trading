package entities

import "time"

// StrategyAdoption is one person putting one published strategy on their own shelf.
//
// It exists because publishing and using are different decisions. Without it, one
// person publishing three strategies would add three entries to everybody else's
// picker, and the picker is a thing people use dozens of times a day. The
// marketplace is the bookshop; this row is the book on your shelf.
//
// The pair is unique, which is what makes adopting twice one row rather than a
// check: the second adoption collides with the first.
//
// It is a plain data model: fields and persistence mapping only. It has no shape of
// its own to hand outwards — what a reader wants is the strategy it points at, not
// the pointing.
type StrategyAdoption struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"not null;index:idx_strategy_adoptions_user;uniqueIndex:idx_strategy_adoptions_user_strategy"`
	// StrategyID names the strategy adopted, and points at the publication rather
	// than at the strategy, so that withdrawing the publication takes this row with
	// it. Adopting something that is not published is therefore not merely refused
	// by a rule — there is no row for it to point at.
	StrategyID uint      `gorm:"not null;uniqueIndex:idx_strategy_adoptions_user_strategy"`
	AdoptedAt  time.Time `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to StrategyAdoptions instead of GORM's default.
func (strategyAdoption StrategyAdoption) TableName() string {
	return "StrategyAdoptions"
}
