package dto

// TelegramDeliveryWriteDto is what somebody hands in to set up delivery to their
// Telegram: the bot to speak as, and the chat to speak into.
//
// It is the one shape in this system that carries a whole bot token, and it travels
// in one direction only — inwards. Nothing converts it back.
//
// There is no field naming whose setting this is. Which account it belongs to comes
// from the proof of identity on the request.
type TelegramDeliveryWriteDto struct {
	BotToken string
	ChatID   string
}
