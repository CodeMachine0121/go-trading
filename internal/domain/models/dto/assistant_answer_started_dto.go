package dto

// AssistantAnswerStartedDto is the immediate reply to a question: the identifiers to watch
// for the answer, not the answer itself.
type AssistantAnswerStartedDto struct {
	ConversationID uint `json:"conversationId"`
	TurnID         uint `json:"turnId"`
	// Status is always running at this point.
	Status string `json:"status"`
}
