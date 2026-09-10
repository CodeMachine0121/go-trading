package messaging

// telegramSendMessageRequest is what Telegram is asked to send.
//
// The chat identifier is text rather than a number, because Telegram accepts both a
// numeric chat and an @name, and turning one into the other here would refuse half
// the identifiers people actually have.
type telegramSendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// telegramSendMessageResponse is the part of Telegram's answer worth keeping.
//
// Description is the sentence it writes for people, and it is the only way to tell
// a rejected token from an unknown chat: both arrive with the same status. Reading
// it is unpleasant and is confined to this file, where it is a wire detail rather
// than a rule.
type telegramSendMessageResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}
