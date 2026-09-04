package dto

// AssistantAskDto is the shape the application hands the domain to ask one question.
//
// A zero conversation identifier names no conversation yet, so it is a question that
// starts one. Anything else names the conversation the question is added to. One
// shape covers both, so the rules about a question are written once instead of twice.
type AssistantAskDto struct {
	ConversationID uint
	Question       string
}
