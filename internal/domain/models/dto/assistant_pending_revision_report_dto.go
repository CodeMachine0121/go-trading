package dto

// AssistantPendingRevisionReportDto is what the assistant reads when its rewrite waits for the owner.
type AssistantPendingRevisionReportDto struct {
	PendingRevisionID uint   `json:"pendingRevisionId"`
	Status            string `json:"status"`
	Notice            string `json:"notice"`
}
