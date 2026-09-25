package dto

import "time"

// TelegramDeliveryDto has no bot token field so no route can leak it; Configured false is a
// normal state, not an error.
type TelegramDeliveryDto struct {
	Configured   bool      `json:"configured"`
	ChatID       string    `json:"chatId"`
	BotTokenTail string    `json:"botTokenTail"`
	ConfiguredAt time.Time `json:"configuredAt"`
}
