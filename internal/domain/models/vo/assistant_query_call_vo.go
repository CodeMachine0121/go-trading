package vo

// AssistantQueryCallVo is the assistant requesting one capability; CallID is echoed back so parallel requests can be matched to results.
type AssistantQueryCallVo struct {
	CallID    string
	Name      string
	Arguments string
}
