package dto

import "time"

type ConversationSummaryDto struct {
	ID           uint      `json:"id"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	MessageCount int       `json:"messageCount"`
}
