package vo

// SignalVo is what an indicator script says about one candle: the opinion a replay
// acts on. Immutable, no behavior — how a raw indicator result becomes one of these
// lives in SignalDomain.
type SignalVo string

const (
	// SignalBuy asks for a long position: open one when flat, reverse into one when
	// short, and do nothing when already long.
	SignalBuy SignalVo = "buy"
	// SignalSell asks for a short position, by the mirror image of those rules.
	SignalSell SignalVo = "sell"
	// SignalFlat asks for nothing at all. It is what a script that never named a
	// signal says on every candle, which is why every script written before signals
	// existed replays as a run with no trades rather than as a failure.
	SignalFlat SignalVo = "flat"
)
