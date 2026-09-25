package domains

// AssistantQueryRoundsDomain counts queries (not round trips, since one trip may request several) against an answer's limit; Record returns a new value.
type AssistantQueryRoundsDomain struct {
	limit int
	used  int
}

func NewAssistantQueryRoundsDomain(limit int) AssistantQueryRoundsDomain {
	return AssistantQueryRoundsDomain{limit: limit, used: 0}
}

func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Allows() bool {
	return assistantQueryRoundsDomain.used < assistantQueryRoundsDomain.limit
}

// ReachedLimit means the assistant must be told to answer now.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) ReachedLimit() bool {
	return assistantQueryRoundsDomain.used >= assistantQueryRoundsDomain.limit
}

// Remaining lets callers allow a prefix of several requested lookups.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Remaining() int {
	remaining := assistantQueryRoundsDomain.limit - assistantQueryRoundsDomain.used
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Used() int {
	return assistantQueryRoundsDomain.used
}

func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Record(count int) AssistantQueryRoundsDomain {
	return AssistantQueryRoundsDomain{
		limit: assistantQueryRoundsDomain.limit,
		used:  assistantQueryRoundsDomain.used + count,
	}
}
