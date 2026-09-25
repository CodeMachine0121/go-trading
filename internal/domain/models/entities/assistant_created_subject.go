package entities

import "time"

// AssistantCreatedSubject remembers that the assistant created a strategy script or trading strategy in a
// conversation, since only those may be rewritten there without the owner's confirmation.
type AssistantCreatedSubject struct {
	ID             uint      `gorm:"primaryKey"`
	ConversationID uint      `gorm:"not null;uniqueIndex:idx_assistant_created_subjects_subject"`
	SubjectKind    string    `gorm:"size:32;not null;uniqueIndex:idx_assistant_created_subjects_subject"`
	SubjectID      uint      `gorm:"not null;uniqueIndex:idx_assistant_created_subjects_subject"`
	CreatedAt      time.Time `gorm:"type:timestamptz;not null"`

	Conversation Conversation `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
}

func (assistantCreatedSubject AssistantCreatedSubject) TableName() string {
	return "AssistantCreatedSubjects"
}
