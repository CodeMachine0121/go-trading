package domains

import (
	"fmt"
	"strings"
)

// AssistantAskDomain is a non-empty question, validated apart from any conversation because a new question has none yet.
type AssistantAskDomain struct {
	question string
}

// NewAssistantAskDomain trims blanks before validating, so an all-blank question is refused.
func NewAssistantAskDomain(question string) (AssistantAskDomain, error) {
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return AssistantAskDomain{}, fmt.Errorf("%w: 必須寫點什麼才問得起來", ErrAssistantAskEmpty)
	}

	return AssistantAskDomain{question: trimmedQuestion}, nil
}

func (assistantAskDomain AssistantAskDomain) Question() string {
	return assistantAskDomain.question
}
