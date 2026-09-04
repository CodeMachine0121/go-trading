package vo

// AssistantTurnRequestVo is everything one round trip to the assistant is given:
// immutable plain data, no behavior.
//
// It is the whole of what the assistant may see. Nothing else reaches it — not the
// conversation's older messages, not what earlier exchanges looked at — so the cost
// of one round trip is bounded by what is assembled here.
type AssistantTurnRequestVo struct {
	// Messages are the recent messages of this conversation, earliest first, with
	// the question being answered last.
	Messages []AssistantMessageVo
	// Declarations are the capabilities the assistant may use. What it can do is
	// this list and nothing more, which is why a capability that is not offered
	// cannot be reached by mistake.
	Declarations []AssistantQueryDeclarationVo
	// Exchanges are what this exchange has already looked at, in the order it
	// happened.
	Exchanges []AssistantQueryExchangeVo
	// QueryLimitReached says no further query will be run, so the assistant is being
	// asked to answer with what it already has.
	QueryLimitReached bool
	// AnswerLengthLimit is how long an answer may be.
	AnswerLengthLimit int
}
