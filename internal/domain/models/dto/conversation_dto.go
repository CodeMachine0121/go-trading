package dto

import "time"

// ConversationDto is one whole conversation as it is handed outwards: every message
// it holds, earliest first.
//
// Every message means every message. Only the most recent handful is ever sent to the
// assistant, but what the assistant can still remember and what a person can still
// read are two different questions, and answering them with one number would make
// the older half of a conversation disappear from the record as well.
type ConversationDto struct {
	ID           uint                     `json:"id"`
	LastActiveAt time.Time                `json:"lastActiveAt"`
	Messages     []ConversationMessageDto `json:"messages"`
}
