package vo

// TargetPositionVo is what a candle's opinion asks the account to be holding once
// that candle is over. Immutable, no behavior — which opinion asks for which target
// is the signal's own answer (see SignalDomain.TargetPosition), and how an account
// gets there lives in BacktestAccountDomain.
//
// It exists so that "what is held" and "what should be held" meet in one value, and
// the walk between them is written once.
type TargetPositionVo string

const (
	// TargetPositionLong asks to be holding a position.
	TargetPositionLong TargetPositionVo = "long"
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

// WantsPosition is whether this target asks the account to be holding something.
//
// Which way is not asked alongside it any more, because there is only one way a spot
// position can face. A replay of contracts brings that question back, and brings its
// own model to answer it.
func (targetPositionVo TargetPositionVo) WantsPosition() bool {
	return targetPositionVo == TargetPositionLong
}
