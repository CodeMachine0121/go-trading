package dto

// AssistantAskDto asks one question; a zero ConversationID starts a new conversation.
type AssistantAskDto struct {
	ConversationID uint
	Question       string
	// ViewerID is who the assistant acts for; strategy scripts it creates belong to, and
	// reads are scoped to, this user.
	ViewerID uint
}
