package vo

// AssistantTurnStatusVo is where one exchange has got to.
//
// There are three and only three. An answer is being written, it was written, or it
// will never be written — and a reader who cannot tell those apart cannot tell "I
// never sent it" from "it is still going" from "it broke", which is the whole reason
// this value exists.
type AssistantTurnStatusVo string

const (
	// AssistantTurnRunning is recorded the moment a question is accepted, before the
	// assistant has been asked anything. It is what makes an answer visible while it
	// is still being written, so a refresh finds it instead of finding nothing.
	AssistantTurnRunning AssistantTurnStatusVo = "running"
	// AssistantTurnAnswered is the assistant having spoken.
	AssistantTurnAnswered AssistantTurnStatusVo = "answered"
	// AssistantTurnFailed is every way an answer ends without one: the assistant
	// being unavailable, a round trip timing out, or the system being restarted
	// while it was mid-thought.
	AssistantTurnFailed AssistantTurnStatusVo = "failed"
)

// NewAssistantTurnStatusVo reads a stored status.
//
// **Nothing at all reads as answered.** That is not a guess: an exchange stored
// before this value existed has no status and an answer in full, so answered is the
// only reading that is true of it. Calling those failed would bury answers people
// already have behind a sentence telling them to ask again.
//
// **Anything else reads as failed.** A value nobody understands could be anything,
// and of the three, failed is the one whose worst case is somebody being told to try
// again. Read as running it would be a wait that never ends, on a conversation that
// can never be asked anything else; read as answered it would show an empty reply as
// though the assistant had said nothing.
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
