package vo

type AssistantMessageRole string

const (
	AssistantMessageRoleAsk    AssistantMessageRole = "ask"
	AssistantMessageRoleAnswer AssistantMessageRole = "answer"
)

// AssistantMessageVo is one asked or answered message; earlier lookups are never resent, which keeps long conversations cheap.
type AssistantMessageVo struct {
	Role    AssistantMessageRole
	Content string
}
