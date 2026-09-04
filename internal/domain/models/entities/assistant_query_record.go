package entities

// AssistantQueryRecord is one assistant query inside one exchange: which capability
// was used, with what arguments, and what came back. It is a plain data model —
// fields and persistence mapping only, no business rules.
//
// It exists so that an answer can be audited after the fact. What the assistant
// looked at is deliberately not sent back to it on the next exchange — that is what
// keeps a long conversation from getting more expensive every time — so this record
// is the only place that knowledge survives at all.
type AssistantQueryRecord struct {
	ID              uint `gorm:"primaryKey"`
	AssistantTurnID uint `gorm:"not null;index:idx_assistant_query_records_turn_id"`
	// Sequence is which query this was within its exchange, counted from one. The
	// order matters: reading them back shuffled makes a chain of reasoning look like
	// a set of unrelated lookups.
	Sequence  int    `gorm:"not null"`
	QueryName string `gorm:"size:64;not null"`
	Arguments string `gorm:"type:text;not null"`
	// Outcome is what was handed back to the assistant, whether that was a result or
	// the reason it was refused. Both are data as far as the assistant is concerned.
	Outcome string `gorm:"type:text;not null"`
	// Rejected tells the two apart for whoever reads this later, which the outcome
	// text alone cannot be trusted to do. Being shown less than was asked for needs
	// no field of its own: the assistant is told so in the outcome itself, because
	// being told is the whole point, and a flag only this table could see would not
	// tell it anything.
	Rejected bool `gorm:"not null"`
}

// TableName pins the table to AssistantQueryRecords instead of GORM's default.
func (assistantQueryRecord AssistantQueryRecord) TableName() string {
	return "AssistantQueryRecords"
}
