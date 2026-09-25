package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TelegramDeliveryRequest names no account, since the account comes from the token.
type TelegramDeliveryRequest struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
}

func (telegramDeliveryRequest TelegramDeliveryRequest) ToTelegramDeliveryWriteDto() dto.TelegramDeliveryWriteDto {
	return dto.TelegramDeliveryWriteDto{
		BotToken: telegramDeliveryRequest.BotToken,
		ChatID:   telegramDeliveryRequest.ChatID,
	}
}
