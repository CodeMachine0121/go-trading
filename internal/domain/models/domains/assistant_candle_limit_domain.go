package domains

import "fmt"

// AssistantCandleLimitDomain caps how many K candles an assistant query may return (far stricter than a person's limit, to bound cost) and flags truncation so the assistant does not mistake a partial stretch for the whole.
type AssistantCandleLimitDomain struct {
	count     int
	truncated bool
}

// NewAssistantCandleLimitDomain treats zero as "use the limit"; there is deliberately no way to ask for everything.
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

func (assistantCandleLimitDomain AssistantCandleLimitDomain) Count() int {
	return assistantCandleLimitDomain.count
}

// Truncated means the assistant must be told it was shown less than it asked for.
func (assistantCandleLimitDomain AssistantCandleLimitDomain) Truncated() bool {
	return assistantCandleLimitDomain.truncated
}
