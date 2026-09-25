package dto

// AssistantRevisionProposalOutcomeDto says whether a proposed rewrite may be written now or was left for the owner.
type AssistantRevisionProposalOutcomeDto struct {
	AppliesDirectly bool
	// PendingRevision is set only when the rewrite waits for the owner.
	PendingRevision AssistantPendingRevisionDto
}
