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
	// QueryCount and StoppedAtQueryLimit describe what an answer cost to write, and
	// appear only on an answer.
	//
	// They travel with the record rather than only with the reply that produced it,
	// because the reply no longer carries anything: a question is answered with a
	// place to look, and this is that place. An answer that ran out of queries is a
	// different thing from a poor one, and a reader who cannot tell them apart has
	// no way to decide whether to ask again more narrowly — so it has to survive here
	// or not at all.
	QueryCount          int  `json:"queryCount,omitempty"`
	StoppedAtQueryLimit bool `json:"stoppedAtQueryLimit,omitempty"`
	// Usage is the share of the assistant this exchange consumed.
	Usage int `json:"usage,omitempty"`
}
