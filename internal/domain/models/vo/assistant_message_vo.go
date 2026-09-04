package vo

// AssistantMessageRole says who said one message in a conversation.
type AssistantMessageRole string

const (
	// AssistantMessageRoleAsk is a message the user wrote.
	AssistantMessageRoleAsk AssistantMessageRole = "ask"
	// AssistantMessageRoleAnswer is a message the assistant wrote.
	AssistantMessageRoleAnswer AssistantMessageRole = "answer"
)

// AssistantMessageVo is one message as the assistant sees it: immutable plain data,
// no behavior. Only what was asked and what was answered ever appear here — what the
// assistant looked at on an earlier exchange is not sent again, which is what keeps a
// long conversation from costing more every time.
type AssistantMessageVo struct {
	Role    AssistantMessageRole
	Content string
}
