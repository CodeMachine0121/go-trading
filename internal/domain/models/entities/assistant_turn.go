package entities

import "time"

// AssistantTurn is one exchange: what was asked, what the assistant answered, where
// that has got to, and the bill for it. It is a plain data model — fields and
// persistence mapping only, no business rules.
//
// The question and the answer live in one row rather than two, because they live and
// die together. The bill and the query count belong to this same exchange, so they
// sit here too rather than in a table that would have to be kept in step with this
// one.
//
// The row is written when the question is accepted, not when the answer is finished.
// An answer that takes minutes exists nowhere else — not in a queue, not in a list
// somebody keeps in memory — so this row **is** the work in progress, and that is
// what lets a refresh find it and a restart sweep up after it. The cost is that a
// failed exchange now leaves a row behind where it used to leave nothing; that is
// deliberate, and Status is how the two are told apart.
type AssistantTurn struct {
	ID             uint   `gorm:"primaryKey"`
	ConversationID uint   `gorm:"not null;index:idx_assistant_turns_conversation_id"`
	Ask            string `gorm:"type:text;not null"`
	// Answer is empty until there is one. Empty and "no answer" are the same thing
	// here, and Status is what says which of the two reasons it is.
	Answer string `gorm:"type:text;not null"`
	// Status is where this exchange has got to: running, answered or failed.
	//
	// Rows written before it existed default to answered, which is the only reading
	// that can be true of them — they all have an answer.
	Status string `gorm:"type:text;not null;default:answered;index:idx_assistant_turns_status"`
	// FailureReason is the one sentence a person reads when an exchange ended without
	// an answer, and is empty on every other row.
	//
	// It is not broken down by kind. Unavailable, timed out and interrupted by a
	// restart all leave the reader with the same thing to do — ask again — so a
	// classification would be a distinction only the log cares about.
	FailureReason string `gorm:"type:text;not null;default:''"`
	// Usage is the share of the assistant this exchange consumed, question and
	// answer together, as the assistant itself reported it. The daily allowance is
	// settled by summing this column, so an exchange that did not record it is an
	// exchange the allowance cannot see.
	//
	// It stays at zero on a failed exchange. Nobody is charged for an answer they
	// never got, and a running one has not reported anything yet — the allowance is
	// settled after the fact, so neither is a hole in it.
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
