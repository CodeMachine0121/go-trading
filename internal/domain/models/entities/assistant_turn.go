package entities

import "time"

// AssistantTurn is written when the question is accepted, so the row itself is the
// in-progress work that a refresh can find and a restart can sweep up.
type AssistantTurn struct {
	ID             uint   `gorm:"primaryKey"`
	ConversationID uint   `gorm:"not null;index:idx_assistant_turns_conversation_id"`
	Ask            string `gorm:"type:text;not null"`
	// Answer is empty until answered; Status says why.
	Answer string `gorm:"type:text;not null"`
	// Status is running, answered or failed; rows predating it default to answered.
	Status string `gorm:"type:text;not null;default:answered;index:idx_assistant_turns_status"`
	// FailureReason is not classified because every failure leaves the reader the same
	// remedy: ask again.
	FailureReason string `gorm:"type:text;not null;default:''"`
	// Usage is summed to settle the daily allowance and stays zero on failed or running exchanges.
	Usage int `gorm:"not null"`
	// QueryCount is stored rather than counted so it survives pruning of the query records.
	QueryCount int `gorm:"not null"`
	// StoppedAtQueryLimit marks a half answer as cut short rather than poor.
	StoppedAtQueryLimit bool `gorm:"not null"`
	// CreatedAt is indexed because the daily allowance sums usage over a time range.
	CreatedAt time.Time              `gorm:"type:timestamptz;not null;index:idx_assistant_turns_created_at"`
	Queries   []AssistantQueryRecord `gorm:"foreignKey:AssistantTurnID;constraint:OnDelete:CASCADE"`
	// PendingRevisions are written as they are proposed and only ever read with the conversation.
	PendingRevisions []AssistantPendingRevision `gorm:"foreignKey:AssistantTurnID;constraint:OnDelete:CASCADE"`
}

func (assistantTurn AssistantTurn) TableName() string {
	return "AssistantTurns"
}
