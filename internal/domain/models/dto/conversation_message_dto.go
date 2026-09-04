package dto

import "time"

// ConversationMessageDto is one message of a conversation as it is handed outwards.
type ConversationMessageDto struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}
