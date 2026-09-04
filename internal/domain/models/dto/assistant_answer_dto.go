package dto

// AssistantAnswerDto is the only shape one exchange leaves the domain in.
//
// The conversation identifier is answered whether or not the caller named one, so
// that a question which started a conversation can be followed up without the caller
// having to go looking for which conversation it landed in.
//
// How many queries ran and whether the assistant ran out of them are reported rather
// than kept private: an answer that stopped early is a different thing from a poor
// answer, and a reader who cannot tell them apart has no way to decide whether to ask
// again more narrowly.
type AssistantAnswerDto struct {
	ConversationID      uint   `json:"conversationId"`
	Answer              string `json:"answer"`
	QueryCount          int    `json:"queryCount"`
	StoppedAtQueryLimit bool   `json:"stoppedAtQueryLimit"`
	Usage               int    `json:"usage"`
}
