package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// TelegramDelivery is where one person wants this system to speak to them: the bot
// it should speak as, and the chat it should speak into. It is a plain data model —
// fields, persistence mapping and shape conversion only, no business rules.
//
// The bot token is not here in the form it is used in. What is stored is the sealed
// form, which nothing outside the delivery path ever opens, plus the last few
// characters kept separately so that reading the setting back never has to open
// anything at all. That split is the point: the one dangerous operation has exactly
// one caller instead of two.
type TelegramDelivery struct {
	ID uint `gorm:"primaryKey"`
	// UserID carries a unique index because one person has at most one of these.
	// The index — not a read-then-write check — is what makes that true: two
	// settings arriving at once both find nothing there, and only one passes an
	// index. It cascades on delete, so a key never outlives the person it belongs
	// to.
	UserID uint `gorm:"not null;uniqueIndex:idx_telegram_deliveries_user_id"`
	// SealedBotToken is the token as stored: locked, not hashed. The difference
	// matters and is the whole reason this is not a password proof — a password is
	// never needed again, and this is needed every time a message goes out, so it
	// has to be openable. What makes that safe is that what opens it is not kept
	// beside it.
	SealedBotToken string `gorm:"size:512;not null"`
	// BotTokenTail is the only part of the token anybody sees again. It is stored
	// rather than derived on read, so that reading a setting never unseals
	// anything.
	BotTokenTail string `gorm:"size:8;not null"`
	// ChatID is stored as it was given. It is not sealed and not masked: it says
	// where a message goes, and holding it alone sends nothing.
	ChatID    string    `gorm:"size:64;not null"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to TelegramDeliveries instead of GORM's default.
func (telegramDelivery TelegramDelivery) TableName() string {
	return "TelegramDeliveries"
}

// ToDto converts this record into the shape the domain hands outwards.
//
// There is no line here dropping the sealed token, because TelegramDeliveryDto has
// nowhere to put one. "The answer never carries the token" is therefore something
// the types make impossible rather than something a reviewer has to keep checking.
func (telegramDelivery TelegramDelivery) ToDto() dto.TelegramDeliveryDto {
	return dto.TelegramDeliveryDto{
		Configured:   true,
		ChatID:       telegramDelivery.ChatID,
		BotTokenTail: telegramDelivery.BotTokenTail,
		ConfiguredAt: telegramDelivery.UpdatedAt,
	}
}
