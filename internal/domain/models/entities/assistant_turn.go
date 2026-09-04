package entities

import "time"

// AssistantTurn is one exchange: what was asked, what the assistant answered, and
// the bill for it. It is a plain data model — fields and persistence mapping only,
// no business rules.
//
// The question and the answer live in one row rather than two, because they live and
// die together: an assistant that never answers leaves nothing behind, and "leaves
// nothing behind" as a single write is a guarantee, while as two writes it is a
// rollback somebody has to remember. The bill and the query count belong to this
// same round trip, so they sit here too rather than in a table that would have to be
// kept in step with this one.
type AssistantTurn struct {
	ID             uint   `gorm:"primaryKey"`
	ConversationID uint   `gorm:"not null;index:idx_assistant_turns_conversation_id"`
	Ask            string `gorm:"type:text;not null"`
	Answer         string `gorm:"type:text;not null"`
	// Usage is the share of the assistant this exchange consumed, question and
	// answer together, as the assistant itself reported it. The daily allowance is
	// settled by summing this column, so an exchange that did not record it is an
	// exchange the allowance cannot see.
	Usage int `gorm:"not null"`
	// QueryCount is how many assistant queries this exchange ran. It is recorded
	// rather than counted from the records below so that the number survives even if
	// what each query did is one day pruned.
	QueryCount int `gorm:"not null"`
	// StoppedAtQueryLimit says whether the assistant ran out of queries before it
	// reached a conclusion, which is what makes a half answer readable as a half
	// answer rather than as a poor one.
	StoppedAtQueryLimit bool `gorm:"not null"`
	// CreatedAt carries an index because the daily allowance is settled by summing
	// usage over a stretch of it.
	CreatedAt time.Time `gorm:"type:timestamptz;not null;index:idx_assistant_turns_created_at"`
	// Queries record what the assistant looked at to produce this answer.
	Queries []AssistantQueryRecord `gorm:"foreignKey:AssistantTurnID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to AssistantTurns instead of GORM's default.
func (assistantTurn AssistantTurn) TableName() string {
	return "AssistantTurns"
}
