package dto

// TelegramDeliveryWriteDto is the only shape holding a whole bot token and is inbound only;
// the owner comes from the request's credentials.
type TelegramDeliveryWriteDto struct {
	BotToken string
	ChatID   string
}
