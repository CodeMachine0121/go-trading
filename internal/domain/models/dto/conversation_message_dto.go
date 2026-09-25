package dto

import "time"

type ConversationMessageDto struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	// Status is running, answered or failed; a running question is the last message and has
	// no reply yet.
	Status        string `json:"status"`
	FailureReason string `json:"failureReason,omitempty"`
	// QueryCount and StoppedAtQueryLimit appear only on answers and are persisted so a
	// reader can tell an answer that ran out of queries from a poor one.
	QueryCount          int  `json:"queryCount,omitempty"`
	StoppedAtQueryLimit bool `json:"stoppedAtQueryLimit,omitempty"`
	Usage               int  `json:"usage,omitempty"`
	// PendingRevisions are the rewrites proposed during this exchange, on its last message only.
	PendingRevisions []AssistantPendingRevisionDto `json:"pendingRevisions,omitempty"`
}
