package entities

import "time"

// Conversation stores LastActiveAt rather than deriving it, since it is the list's only sort key.
type Conversation struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is not nullable because transcripts can contain the owner's private strategy scripts.
	OwnerID uint `gorm:"not null;index:idx_conversations_owner"`
	// LastActiveAt has a descending index because it is the only order conversations are listed in.
	LastActiveAt time.Time `gorm:"type:timestamptz;not null;index:idx_conversations_last_active_at,sort:desc"`
	CreatedAt    time.Time `gorm:"type:timestamptz;not null"`
	// Turns have no repository of their own; they are only ever read, created and deleted
	// with their conversation.
	Turns []AssistantTurn `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
}

func (conversation Conversation) TableName() string {
	return "Conversations"
}
