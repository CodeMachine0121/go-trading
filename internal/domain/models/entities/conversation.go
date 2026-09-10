package entities

import "time"

// Conversation is one conversation: an identity, when it last had something added to
// it, and the exchanges that have accumulated under it. It is a plain data model —
// fields and persistence mapping only, no business rules.
//
// When it was last active is stored rather than worked out from its exchanges. The
// list of conversations is ordered by it and nothing else, and an order that has to
// read every exchange of every conversation to decide it is an order that gets
// slower for no reason a reader could name.
type Conversation struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is who asked. A conversation belongs to one person for the same reason
	// a strategy does, and here the reason is sharper: the assistant acts as whoever
	// asked it, so a transcript can hold that person's own strategies — the very
	// thing publishing exists to hand out deliberately rather than by accident.
	//
	// It is not nullable. A conversation with nobody behind it could be read by
	// everybody, and "belongs to one person" would then be true only of the rows
	// that happened to be written after this column arrived.
	OwnerID uint `gorm:"not null;index:idx_conversations_owner"`
	// LastActiveAt carries a descending index because it is the only order the list
	// of conversations is ever read in.
	LastActiveAt time.Time `gorm:"type:timestamptz;not null;index:idx_conversations_last_active_at,sort:desc"`
	CreatedAt    time.Time `gorm:"type:timestamptz;not null"`
	// Turns belong to this conversation and to nothing else: an exchange is never
	// read, created or deleted on its own, which is why it has no repository of its
	// own and travels with the conversation that owns it.
	Turns []AssistantTurn `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to Conversations instead of GORM's default conversations.
func (conversation Conversation) TableName() string {
	return "Conversations"
}
