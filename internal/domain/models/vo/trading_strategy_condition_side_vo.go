package vo

// TradingStrategyConditionSideVo says which of the two condition trees (buy or sell) a stored node belongs to.
type TradingStrategyConditionSideVo string

const (
	TradingStrategyConditionSideBuy  TradingStrategyConditionSideVo = "buy"
	TradingStrategyConditionSideSell TradingStrategyConditionSideVo = "sell"
)
