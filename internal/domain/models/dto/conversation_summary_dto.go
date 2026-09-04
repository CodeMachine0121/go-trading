package dto

import "time"

// ConversationSummaryDto is one conversation as it appears in the list of them:
// enough to recognise and pick, without carrying every message along.
type ConversationSummaryDto struct {
	ID           uint      `json:"id"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	MessageCount int       `json:"messageCount"`
}
