package vo

// AssistantQueryExchangeVo is one request-and-result pair from the exchange that is
// still in progress: immutable plain data, no behavior.
//
// These accumulate within a single exchange and are sent back to the assistant on
// every round of it, because that is how it sees what it has already learned. They
// are dropped once the exchange ends and are never sent again.
type AssistantQueryExchangeVo struct {
	Call     AssistantQueryCallVo
	Outcome  string
	Rejected bool
}
