package domains

// AssistantQueryRoundsDomain counts the assistant queries one answer has spent and
// says when there are no more to spend.
//
// It counts queries rather than round trips, because a round trip that asks for three
// lookups costs three lookups. Counting trips would let one answer read the whole
// market in a single trip and still call it one.
//
// Recording hands back a new value rather than changing this one: whether a query may
// run is asked and answered several times within one answer, and a counter that
// changes under the asker is a counter nobody can reason about.
type AssistantQueryRoundsDomain struct {
	limit int
	used  int
}

// NewAssistantQueryRoundsDomain starts the count for one answer.
func NewAssistantQueryRoundsDomain(limit int) AssistantQueryRoundsDomain {
	return AssistantQueryRoundsDomain{limit: limit, used: 0}
}

// Allows says whether one more assistant query may run.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Allows() bool {
	return assistantQueryRoundsDomain.used < assistantQueryRoundsDomain.limit
}

// ReachedLimit says the allowance for this answer is spent. It is what turns a
// half-finished answer into an honest one: the assistant is told it must speak now.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) ReachedLimit() bool {
	return assistantQueryRoundsDomain.used >= assistantQueryRoundsDomain.limit
}

// Remaining is how many more assistant queries this answer may spend.
//
// It exists because the assistant may ask for several lookups in one breath, and
// the honest reply to "may I run these five?" is "you may run the first two" —
// not a yes or a no.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Remaining() int {
	remaining := assistantQueryRoundsDomain.limit - assistantQueryRoundsDomain.used
	if remaining < 0 {
		return 0
	}

	return remaining
}

// Used is how many assistant queries this answer has spent.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Used() int {
	return assistantQueryRoundsDomain.used
}

// Record counts this many assistant queries as spent, handing back the count as it
// now stands.
func (assistantQueryRoundsDomain AssistantQueryRoundsDomain) Record(count int) AssistantQueryRoundsDomain {
	return AssistantQueryRoundsDomain{
		limit: assistantQueryRoundsDomain.limit,
		used:  assistantQueryRoundsDomain.used + count,
	}
}
