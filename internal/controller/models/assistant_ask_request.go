package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// AssistantAskRequest names its conversation in the body because omitting it, the common case, starts a new one.
type AssistantAskRequest struct {
	ConversationID uint   `json:"conversationId"`
	Question       string `json:"question"`
}

// ToAskDto takes the asker from the argument, never the body, so a caller cannot ask as someone else.
func (assistantAskRequest AssistantAskRequest) ToAskDto(viewerID uint) dto.AssistantAskDto {
	return dto.AssistantAskDto{
		ConversationID: assistantAskRequest.ConversationID,
		Question:       assistantAskRequest.Question,
		ViewerID:       viewerID,
	}
}
