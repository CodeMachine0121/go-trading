package vo

// StrategyBotConditionSideVo says which of a bot's two condition trees a stored node
// belongs to.
//
// A bot has exactly two conditions and they never grow to three: a bot has two
// exits, buy and sell, and anything else it might conclude is the absence of both.
// So this is a column on the shared node table rather than two tables of identical
// shape, which would mean writing every query twice.
type StrategyBotConditionSideVo string

const (
	// StrategyBotConditionSideBuy is the tree that says when this bot thinks it
	// should buy.
	StrategyBotConditionSideBuy StrategyBotConditionSideVo = "buy"
	// StrategyBotConditionSideSell is the tree that says when it thinks it should
	// sell.
	StrategyBotConditionSideSell StrategyBotConditionSideVo = "sell"
)
