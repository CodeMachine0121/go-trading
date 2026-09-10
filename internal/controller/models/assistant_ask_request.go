package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// AssistantAskRequest is the body a caller sends to ask the assistant one question.
//
// Which conversation is meant comes from the body rather than the path, because the
// common case is that there is not one yet: a question that names none starts one,
// and a path cannot name something that does not exist. Leaving it out is therefore
// the ordinary way to use this, not an omission.
type AssistantAskRequest struct {
	ConversationID uint   `json:"conversationId"`
	Question       string `json:"question"`
}

// ToAskDto turns the request into the shape the domain accepts, taking who is
// asking from the argument rather than from the body — a body that could name its
// own asker is a body that could ask as somebody else.
//
// The question is
// handed on untouched: what counts as an empty question is the domain's rule, not
// this layer's.
func (assistantAskRequest AssistantAskRequest) ToAskDto(viewerID uint) dto.AssistantAskDto {
	return dto.AssistantAskDto{
		ConversationID: assistantAskRequest.ConversationID,
		Question:       assistantAskRequest.Question,
		ViewerID:       viewerID,
	}
}
