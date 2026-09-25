package vo

// AssistantTurnRequestVo is everything the assistant sees in one round trip, which bounds its cost.
type AssistantTurnRequestVo struct {
	// Messages are the recent messages, earliest first, with the current question last.
	Messages []AssistantMessageVo
	// Declarations are the only capabilities the assistant can reach.
	Declarations []AssistantQueryDeclarationVo
	// Rounds are this exchange's earlier lookup rounds, in order.
	Rounds []AssistantQueryRoundVo
	// QueryLimitReached asks the assistant to answer with what it already has.
	QueryLimitReached bool
	AnswerLengthLimit int
}
