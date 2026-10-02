package entities

import "time"

// PendingMessage is one message a bot has to say, written in the same transaction as what caused it and sent afterwards by whichever replica takes it.
type PendingMessage struct {
	ID uint `gorm:"primaryKey"`
	// StrategyBotID with RoundDueAt identifies a round, so a round can never queue two messages; a bot's messages about itself leave RoundDueAt empty, and empties never collide.
	StrategyBotID   uint       `gorm:"not null;uniqueIndex:idx_pending_messages_bot_round,priority:1"`
	RoundDueAt      *time.Time `gorm:"type:timestamptz;uniqueIndex:idx_pending_messages_bot_round,priority:2"`
	RecipientUserID uint       `gorm:"not null;index:idx_pending_messages_unsettled,priority:2"`
	Kind            string     `gorm:"size:16;not null"`
	// Signal is the round's sent signal, forgotten by the bot if this message is never delivered.
	Signal string `gorm:"size:16;not null;default:''"`
	Text   string `gorm:"type:text;not null"`
	Status string `gorm:"size:16;not null;index:idx_pending_messages_unsettled,priority:1"`
	// AttemptCount counts failed attempts and sets how long the next wait is.
	AttemptCount  int       `gorm:"not null;default:0"`
	NextAttemptAt time.Time `gorm:"type:timestamptz;not null"`
	// ClaimedBy and ClaimedUntil say which replica is sending it; a claim past its time means that replica died mid-send and anyone may send it again.
	ClaimedBy     string     `gorm:"size:255;not null;default:''"`
	ClaimedUntil  *time.Time `gorm:"type:timestamptz"`
	ExpiresAt     time.Time  `gorm:"type:timestamptz;not null"`
	SettledAt     *time.Time `gorm:"type:timestamptz;index:idx_pending_messages_settled_at"`
	AbandonReason string     `gorm:"size:32;not null;default:''"`
	CreatedAt     time.Time  `gorm:"type:timestamptz;not null"`
}

func (pendingMessage PendingMessage) TableName() string {
	return "PendingMessages"
}
