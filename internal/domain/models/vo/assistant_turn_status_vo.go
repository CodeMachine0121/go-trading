package vo

// AssistantTurnStatusVo distinguishes an answer in progress, answered, or never coming.
type AssistantTurnStatusVo string

const (
	// AssistantTurnRunning is recorded on acceptance so a refresh finds the in-progress answer.
	AssistantTurnRunning  AssistantTurnStatusVo = "running"
	AssistantTurnAnswered AssistantTurnStatusVo = "answered"
	// AssistantTurnFailed covers an unavailable assistant, a timeout, or a restart mid-answer.
	AssistantTurnFailed AssistantTurnStatusVo = "failed"
)

// NewAssistantTurnStatusVo reads a stored status: empty means answered (rows predating statuses have full answers), and anything unknown means failed, the safest reading.
func NewAssistantTurnStatusVo(status string) AssistantTurnStatusVo {
	switch AssistantTurnStatusVo(status) {
	case "", AssistantTurnAnswered:
		return AssistantTurnAnswered
	case AssistantTurnRunning:
		return AssistantTurnRunning
	default:
		return AssistantTurnFailed
	}
}
