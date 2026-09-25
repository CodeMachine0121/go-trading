package vo

// AssistantQueryRoundVo is one lookup round kept whole: its narration must come back with its lookups or the assistant loses its train of thought, and parallel lookups must return as one reply.
type AssistantQueryRoundVo struct {
	// Narration is what the assistant said alongside the requests; it is not the answer.
	Narration string
	Exchanges []AssistantQueryExchangeVo
}
