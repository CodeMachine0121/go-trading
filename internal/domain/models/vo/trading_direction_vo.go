package vo

// TradingDirectionVo is one direction a contract bot may face; a bot picks one or both.
type TradingDirectionVo string

const (
	TradingDirectionLong  TradingDirectionVo = "long"
	TradingDirectionShort TradingDirectionVo = "short"
)
