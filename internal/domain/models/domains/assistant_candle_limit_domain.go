package domains

import "fmt"

// AssistantCandleLimitDomain settles how many K candles one assistant query may hand
// over, and whether the assistant is being shown less than it asked for.
//
// The limit is deliberately far stricter than the one a person's own query obeys.
// The two exist for different reasons: a person's limit is about what a response can
// carry, this one is about what an answer costs. It narrows the person's limit and
// never replaces it.
//
// Being told about the truncation matters as much as the truncation. An assistant
// shown the most recent two hundred of five hundred candles, and not told, will read
// them as the whole stretch and describe a trend that does not exist.
type AssistantCandleLimitDomain struct {
	count     int
	truncated bool
}

// NewAssistantCandleLimitDomain reads how many candles the assistant asked for against
// the limit in force.
//
// Asking for none is not asking for everything — it is not saying, so the limit
// itself is used and nothing is truncated. There is deliberately no way to ask for
// everything: the whole point of the limit is that no single question can decide how
// much an answer costs.
func NewAssistantCandleLimitDomain(limit int, requestedCount int) (AssistantCandleLimitDomain, error) {
	if requestedCount < 0 {
		return AssistantCandleLimitDomain{}, fmt.Errorf(
			"%w: 根數必須大於零", ErrAssistantQueryArgument)
	}

	if requestedCount == 0 {
		return AssistantCandleLimitDomain{count: limit, truncated: false}, nil
	}

	if requestedCount > limit {
		return AssistantCandleLimitDomain{count: limit, truncated: true}, nil
	}

	return AssistantCandleLimitDomain{count: requestedCount, truncated: false}, nil
}

// Count is how many candles may actually be handed over.
func (assistantCandleLimitDomain AssistantCandleLimitDomain) Count() int {
	return assistantCandleLimitDomain.count
}

// Truncated says the assistant is being shown less than it asked for, and must be
// told so.
func (assistantCandleLimitDomain AssistantCandleLimitDomain) Truncated() bool {
	return assistantCandleLimitDomain.truncated
}
