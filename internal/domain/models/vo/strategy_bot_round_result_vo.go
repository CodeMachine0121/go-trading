package vo

// StrategyBotRoundResultVo is one round as its history remembers it.
//
// It is **not** a signal, and that is why it is its own type rather than a reuse of
// SignalVo. A signal is what a strategy read off the market; this is what a whole
// round came to, and one of its values — conflicted — is something no strategy can
// ever say.
//
// Four values rather than three. Conflicted used to be recorded as hold, on the
// reasoning that a reader only cares whether the bot took a position. That was
// wrong in the one way that matters: a conflicted bot will stay silent for ever
// until its owner goes and changes a condition, while a holding bot is simply
// waiting. One of those is a thing to go and fix; showing them as the same word
// hides the only entry in the history that asks for action.
//
// Immutable, no behavior: how a verdict becomes one of these lives in
// StrategyBotRoundOutcomeDomain.
type StrategyBotRoundResultVo string

const (
	// StrategyBotRoundResultBuy is a round that concluded the bot should buy.
	StrategyBotRoundResultBuy StrategyBotRoundResultVo = "buy"
	// StrategyBotRoundResultSell is the mirror of that.
	StrategyBotRoundResultSell StrategyBotRoundResultVo = "sell"
	// StrategyBotRoundResultHold is every ordinary quiet round: neither condition
	// held, or the round never got far enough to decide.
	StrategyBotRoundResultHold StrategyBotRoundResultVo = "hold"
	// StrategyBotRoundResultConflict is both conditions holding at once. The bot is
	// running and healthy and will say nothing at all until a condition changes,
	// which is why it is worth a word of its own in the history.
	StrategyBotRoundResultConflict StrategyBotRoundResultVo = "conflict"
)
