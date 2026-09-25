package vo

// StrategyBotRoundResultVo is what a whole round came to; unlike SignalVo it includes conflict, which needs the owner's attention. See StrategyBotRoundOutcomeDomain.
type StrategyBotRoundResultVo string

const (
	StrategyBotRoundResultBuy  StrategyBotRoundResultVo = "buy"
	StrategyBotRoundResultSell StrategyBotRoundResultVo = "sell"
	// StrategyBotRoundResultHold covers neither condition holding and rounds that never reached a decision.
	StrategyBotRoundResultHold StrategyBotRoundResultVo = "hold"
	// StrategyBotRoundResultConflict is both conditions holding; the bot stays silent until a condition changes.
	StrategyBotRoundResultConflict StrategyBotRoundResultVo = "conflict"
)
