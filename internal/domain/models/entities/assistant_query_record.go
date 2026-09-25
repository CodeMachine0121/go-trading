package entities

// AssistantQueryRecord exists for auditing, since what the assistant looked at is
// deliberately not replayed into later exchanges.
type AssistantQueryRecord struct {
	ID              uint `gorm:"primaryKey"`
	AssistantTurnID uint `gorm:"not null;index:idx_assistant_query_records_turn_id"`
	// Sequence counts from one and preserves the order of the reasoning chain.
	Sequence  int    `gorm:"not null"`
	QueryName string `gorm:"size:64;not null"`
	Arguments string `gorm:"type:text;not null"`
	// Outcome is the result or refusal reason handed back to the assistant.
	Outcome string `gorm:"type:text;not null"`
	// Rejected distinguishes a refusal from a result, which the outcome text alone cannot be
	// trusted to do.
	Rejected bool `gorm:"not null"`
}

func (assistantQueryRecord AssistantQueryRecord) TableName() string {
	return "AssistantQueryRecords"
}
