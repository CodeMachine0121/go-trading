package vo

// SpotTradeStatusVo has no planned state: a spot trade exists only once its first buy does.
type SpotTradeStatusVo string

const (
	SpotTradeStatusOpen     SpotTradeStatusVo = "open"
	SpotTradeStatusClosed   SpotTradeStatusVo = "closed"
	SpotTradeStatusReviewed SpotTradeStatusVo = "reviewed"
)
