package vo

// ConditionOperatorVo is how a condition group joins its children; evaluation lives in TradingStrategyConditionDomain.
type ConditionOperatorVo string

const (
	ConditionOperatorAnd ConditionOperatorVo = "and"
	ConditionOperatorOr  ConditionOperatorVo = "or"
)
