package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TestMessageRequest carries only the message; the bot and chat come from the stored setting so a bot token has no second way in.
type TestMessageRequest struct {
	Message string `json:"message"`
}

func (testMessageRequest TestMessageRequest) ToTestMessageDto() dto.TestMessageDto {
	return dto.TestMessageDto{Message: testMessageRequest.Message}
}
