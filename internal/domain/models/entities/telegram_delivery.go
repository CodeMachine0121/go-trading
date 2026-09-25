package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// TelegramDelivery is one person's Telegram bot and chat; the token is stored sealed with its tail kept separately so reads never unseal it.
type TelegramDelivery struct {
	ID uint `gorm:"primaryKey"`
	// UserID is uniquely indexed (one delivery per person) and cascades on delete.
	UserID uint `gorm:"not null;uniqueIndex:idx_telegram_deliveries_user_id"`
	// SealedBotToken is encrypted rather than hashed because every send needs it back.
	SealedBotToken string `gorm:"size:512;not null"`
	// BotTokenTail is stored so reading the setting never unseals the token.
	BotTokenTail string    `gorm:"size:8;not null"`
	ChatID       string    `gorm:"size:64;not null"`
	CreatedAt    time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt    time.Time `gorm:"type:timestamptz;not null"`
}

func (telegramDelivery TelegramDelivery) TableName() string {
	return "TelegramDeliveries"
}

// ToDto cannot leak the sealed token because TelegramDeliveryDto has no field for it.
func (telegramDelivery TelegramDelivery) ToDto() dto.TelegramDeliveryDto {
	return dto.TelegramDeliveryDto{
		Configured:   true,
		ChatID:       telegramDelivery.ChatID,
		BotTokenTail: telegramDelivery.BotTokenTail,
		ConfiguredAt: telegramDelivery.UpdatedAt,
	}
}
