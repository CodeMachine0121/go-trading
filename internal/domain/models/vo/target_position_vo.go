package vo

// TargetPositionVo is what a candle's opinion asks the account to be holding once
// that candle is over. Immutable, no behavior — which opinion asks for which target
// lives in TradingModeDomain, and how an account gets there lives in
// BacktestAccountDomain.
//
// It exists so that the two ways of trading differ in one value rather than in two
// copies of the walk from "what is held" to "what should be held".
type TargetPositionVo string

const (
	// TargetPositionLong asks to be holding a long position.
	TargetPositionLong TargetPositionVo = "long"
	// TargetPositionShort asks to be holding a short position.
	TargetPositionShort TargetPositionVo = "short"
	// TargetPositionFlat asks to be holding nothing: close whatever is open and stay
	// in cash.
	//
	// It is not the same as TargetPositionUnchanged, and collapsing the two would be
	// a mistake nobody would notice: unchanged is having no opinion, flat is having
	// one and asking to get out.
	TargetPositionFlat TargetPositionVo = "flat"
	// TargetPositionUnchanged asks for nothing at all: whatever is held stays held.
	TargetPositionUnchanged TargetPositionVo = "unchanged"
)

// WantedDirection is which way this target asks the account to face, and whether it
// asks for a position at all. Cash and having no opinion both ask for none.
//
// Answering both in one call is what keeps a caller from asking "is it flat" and then
// asking again which way — two questions that can only ever be answered together.
func (targetPositionVo TargetPositionVo) WantedDirection() (PositionDirectionVo, bool) {
	if targetPositionVo == TargetPositionLong {
		return PositionDirectionLong, true
	}
	if targetPositionVo == TargetPositionShort {
		return PositionDirectionShort, true
	}

	return "", false
}
