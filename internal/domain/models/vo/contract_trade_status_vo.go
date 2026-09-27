package vo

// ContractTradeStatusVo has no planned state: a trade exists only once its first entry fill does.
type ContractTradeStatusVo string

const (
	ContractTradeStatusOpen     ContractTradeStatusVo = "open"
	ContractTradeStatusClosed   ContractTradeStatusVo = "closed"
	ContractTradeStatusReviewed ContractTradeStatusVo = "reviewed"
)
