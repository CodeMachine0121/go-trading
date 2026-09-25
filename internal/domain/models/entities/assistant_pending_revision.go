package entities

import (
	"encoding/json"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// AssistantPendingRevision is written the moment the assistant proposes a rewrite, not when its answer ends, so a
// proposal survives an answer that later fails.
type AssistantPendingRevision struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is the asker, copied so confirming needs no read of the conversation.
	OwnerID         uint   `gorm:"not null;index:idx_assistant_pending_revisions_owner"`
	ConversationID  uint   `gorm:"not null"`
	AssistantTurnID uint   `gorm:"not null;index:idx_assistant_pending_revisions_turn"`
	SubjectKind     string `gorm:"size:32;not null"`
	SubjectID       uint   `gorm:"not null"`
	// SubjectName is the name when proposed, so the proposal still reads sensibly after a rename.
	SubjectName string `gorm:"size:128;not null"`
	// Content is the assistant's arguments verbatim: what the owner reviews is exactly what would be written.
	Content string `gorm:"type:text;not null"`
	// SubjectUpdatedAt is when the subject was last changed as of the proposal; any later change voids it.
	SubjectUpdatedAt time.Time `gorm:"type:timestamptz;not null"`
	Status           string    `gorm:"size:16;not null"`
	ProposedAt       time.Time `gorm:"type:timestamptz;not null"`
}

func (assistantPendingRevision AssistantPendingRevision) TableName() string {
	return "AssistantPendingRevisions"
}

func (assistantPendingRevision AssistantPendingRevision) ToDto() dto.AssistantPendingRevisionDto {
	return dto.AssistantPendingRevisionDto{
		ID:               assistantPendingRevision.ID,
		SubjectKind:      assistantPendingRevision.SubjectKind,
		SubjectID:        assistantPendingRevision.SubjectID,
		SubjectName:      assistantPendingRevision.SubjectName,
		Content:          json.RawMessage(assistantPendingRevision.Content),
		Status:           assistantPendingRevision.Status,
		ProposedAt:       assistantPendingRevision.ProposedAt.UTC(),
		SubjectUpdatedAt: assistantPendingRevision.SubjectUpdatedAt.UTC(),
	}
}
