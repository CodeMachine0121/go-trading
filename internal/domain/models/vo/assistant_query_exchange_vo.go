package vo

// AssistantQueryExchangeVo is one request/result pair, resent on every round of the current exchange and dropped once it ends.
type AssistantQueryExchangeVo struct {
	Call     AssistantQueryCallVo
	Outcome  string
	Rejected bool
}
