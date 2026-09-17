package dto

// AssistantAnswerStartedDto is what a question is answered with immediately: a place
// has been reserved for the answer, and this is where to find it.
//
// It is not the answer. The answer takes as long as it takes — the assistant may go
// round forty times building a set of rules and replaying it — and holding the asker
// on the line for that is what this whole design removes. What comes back at once is
// the two identifiers needed to watch for it and the state it starts in.
//
// The conversation identifier is answered whether or not the caller named one, so
// that a question which started a conversation can be followed without going looking
// for where it landed.
type AssistantAnswerStartedDto struct {
	ConversationID uint `json:"conversationId"`
	// TurnID names the exchange whose answer is being written.
	TurnID uint `json:"turnId"`
	// Status is where it has got to, which at this moment is always running. It is
	// reported rather than assumed so that a reader has one field to look at
	// throughout, whether it is reading this or reading the conversation later.
	Status string `json:"status"`
}
