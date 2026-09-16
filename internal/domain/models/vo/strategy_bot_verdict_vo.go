package vo

// StrategyBotVerdictVo is what one round of a strategy bot concluded.
//
// Four values rather than three, because "the two conditions disagree" is not the
// same as "neither said anything" even though both stay silent. They are told apart
// so that the first can be shown to the owner as something to go and fix, while the
// second is simply an ordinary quiet round.
//
// Immutable, no behavior: how two conditions become one of these, and whether it is
// worth speaking about, lives in StrategyBotVerdictDomain.
type StrategyBotVerdictVo string

const (
	// StrategyBotVerdictBuy is the buy condition holding while the sell one does not.
	StrategyBotVerdictBuy StrategyBotVerdictVo = "buy"
	// StrategyBotVerdictSell is the mirror of that.
	StrategyBotVerdictSell StrategyBotVerdictVo = "sell"
	// StrategyBotVerdictNone is neither condition holding. It is not an opinion, so
	// it never displaces the last signal that was sent.
	StrategyBotVerdictNone StrategyBotVerdictVo = "none"
	// StrategyBotVerdictConflict is both conditions holding at once: this bot
	// thinks it should buy and sell in the same breath. Nothing is sent and no side
	// is picked — choosing one would hand somebody an opinion the system made up,
	// and they would never find out.
	StrategyBotVerdictConflict StrategyBotVerdictVo = "conflict"
)
