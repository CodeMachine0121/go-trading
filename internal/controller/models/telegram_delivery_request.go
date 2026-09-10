package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TelegramDeliveryRequest is the body of a request to say where this system should
// speak to somebody.
//
// It names no account. Which one this belongs to comes from the proof of identity
// on the request, so there is no field somebody could fill in with a stranger's
// identifier — and therefore no check to remember writing.
type TelegramDeliveryRequest struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
}

// ToTelegramDeliveryWriteDto is this request in the shape the domain accepts.
func (telegramDeliveryRequest TelegramDeliveryRequest) ToTelegramDeliveryWriteDto() dto.TelegramDeliveryWriteDto {
	return dto.TelegramDeliveryWriteDto{
		BotToken: telegramDeliveryRequest.BotToken,
		ChatID:   telegramDeliveryRequest.ChatID,
	}
}
