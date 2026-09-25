package dto

// AssistantRevisionProposalDto is one rewrite the assistant wants to make, already checked under the rewrite rules.
type AssistantRevisionProposalDto struct {
	ViewerID       uint
	ConversationID uint
	TurnID         uint
	SubjectKind    string
	Target         RewriteTargetDto
	Content        string
}
