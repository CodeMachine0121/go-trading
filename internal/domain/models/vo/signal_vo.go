package vo

// SignalVo is what an indicator script says about one candle: the opinion a replay
// acts on. It is the whole content of an indicator value under the signal result
// kind. Immutable, no behavior — how the script runner's raw output becomes one of
// these, and how a replay reads it, lives in SignalDomain.
type SignalVo string

const (
	// SignalBuy asks for a long position: open one when holding nothing, reverse
	// into one when short, and do nothing when already long.
	SignalBuy SignalVo = "buy"
	// SignalSell asks for a short position, by the mirror image of those rules.
	SignalSell SignalVo = "sell"
	// SignalHold asks for nothing at all: the position, if any, stays as it is and
	// no trade is recorded.
	SignalHold SignalVo = "hold"
)

// SignalIndicatorKey is the key the script runner files the one signal under when a
// calculation declares the signal result kind. It is internal plumbing — never a
// user-facing indicator name — but it is a named constant rather than a literal
// buried in two layers, because renaming it must be one visible line.
const SignalIndicatorKey = "signal"
