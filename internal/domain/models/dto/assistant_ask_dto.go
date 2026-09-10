package dto

// AssistantAskDto is the shape the application hands the domain to ask one question.
//
// A zero conversation identifier names no conversation yet, so it is a question that
// starts one. Anything else names the conversation the question is added to. One
// shape covers both, so the rules about a question are written once instead of twice.
type AssistantAskDto struct {
	ConversationID uint
	Question       string
	// ViewerID is who the assistant is acting for. Everything it does with
	// strategies it does as this person: what it creates belongs to them, and what
	// it can read is what they can read. Without it, an assistant asked to save a
	// strategy would produce one belonging to nobody — and "every strategy has an
	// owner" would have its first exception.
	ViewerID uint
}
