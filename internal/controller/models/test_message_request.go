package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TestMessageRequest is the body of a request to send one message to somebody's own
// Telegram, to find out whether that route works at all.
//
// It carries the message and nothing else — not the bot, not the chat. Those come
// from the setting already stored, which is what keeps a whole bot token from having
// a second way into this system.
type TestMessageRequest struct {
	Message string `json:"message"`
}

// ToTestMessageDto is this request in the shape the domain accepts.
func (testMessageRequest TestMessageRequest) ToTestMessageDto() dto.TestMessageDto {
	return dto.TestMessageDto{Message: testMessageRequest.Message}
}
