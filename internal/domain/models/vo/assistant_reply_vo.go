package vo

// AssistantReplyVo is either an answer or a request for capabilities; usage is reported either way.
type AssistantReplyVo struct {
	Answer     string
	QueryCalls []AssistantQueryCallVo
	Usage      int
}
