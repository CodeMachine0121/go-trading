package vo

// AssistantReplyVo is what one round trip to the assistant came back with:
// immutable plain data, no behavior.
//
// Exactly one of the two halves is meant at a time: either the assistant answered,
// or it wants capabilities run first. Usage is reported either way, because a round
// trip that only asked for a lookup still cost something.
type AssistantReplyVo struct {
	Answer     string
	QueryCalls []AssistantQueryCallVo
	Usage      int
}
