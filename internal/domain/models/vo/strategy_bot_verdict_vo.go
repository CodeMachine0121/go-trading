package vo

// StrategyBotVerdictVo is what one round concluded; conflict is kept apart from none so it can be surfaced to the owner. See StrategyBotVerdictDomain.
type StrategyBotVerdictVo string

const (
	// StrategyBotVerdictBuy is the buy condition holding while the sell one does not.
	StrategyBotVerdictBuy  StrategyBotVerdictVo = "buy"
	StrategyBotVerdictSell StrategyBotVerdictVo = "sell"
	// StrategyBotVerdictNone never displaces the last signal that was sent.
	StrategyBotVerdictNone StrategyBotVerdictVo = "none"
	// StrategyBotVerdictConflict sends nothing and picks no side.
	StrategyBotVerdictConflict StrategyBotVerdictVo = "conflict"
)
