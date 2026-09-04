package domains

import (
	"fmt"
	"strings"
)

// AssistantAskDomain holds one question and guarantees it is worth answering. An
// instance only exists when the question said something, so there is no half-valid
// question further in.
//
// It stands apart from the conversation it will be added to on purpose. A question
// that names no conversation has to be judged before any conversation exists, and a
// rule that lives on the conversation could not be reached until one had been
// conjured up just to have somewhere to put the rule.
type AssistantAskDomain struct {
	question string
}

// NewAssistantAskDomain reads a question, dropping the blanks around it. Blanks are
// dropped before the question is judged, so that a question made only of them is
// refused rather than stored as a question nobody can read.
func NewAssistantAskDomain(question string) (AssistantAskDomain, error) {
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return AssistantAskDomain{}, fmt.Errorf("%w: 必須寫點什麼才問得起來", ErrAssistantAskEmpty)
	}

	return AssistantAskDomain{question: trimmedQuestion}, nil
}

// Question is the question as it will be stored and sent: without the blanks it
// arrived wrapped in.
func (assistantAskDomain AssistantAskDomain) Question() string {
	return assistantAskDomain.question
}
