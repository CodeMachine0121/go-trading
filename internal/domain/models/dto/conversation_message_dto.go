package dto

import "time"

// ConversationMessageDto is one message of a conversation as it is handed outwards.
//
// It carries the state of the exchange it belongs to, not just what was said. A
// conversation read back can hold an exchange still being written and one that
// failed, and a reader with only the words has no way to tell either from an answer
// that simply has not been given.
type ConversationMessageDto struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	// Status is where this message's exchange has got to: running, answered or
	// failed. A question whose exchange is still running is the last message of the
	// conversation and has no reply after it yet — that is what a screen draws a wait
	// on.
	Status string `json:"status"`
	// FailureReason is the one sentence explaining an exchange that ended without an
	// answer, and is absent on every other message.
	FailureReason string `json:"failureReason,omitempty"`
}
