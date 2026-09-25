package dto

import "time"

// ConversationDto holds every message, earliest first, even though only the most recent few
// are sent to the assistant.
type ConversationDto struct {
	ID           uint                     `json:"id"`
	LastActiveAt time.Time                `json:"lastActiveAt"`
	Messages     []ConversationMessageDto `json:"messages"`
}
