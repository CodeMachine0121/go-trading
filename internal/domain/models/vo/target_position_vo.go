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
	// TargetPositionShort asks to be holding a short position. Only a contract trading
	// mode ever asks for it (see ContractTradingModeDomain.TargetFor).
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

// WantsPosition is whether this target asks the account to be holding something.
//
// It answers for the spot account, where the only position is a long one. The contract
// account asks which way a target faces instead (see ContractBacktestAccountDomain.Apply).
func (targetPositionVo TargetPositionVo) WantsPosition() bool {
	return targetPositionVo == TargetPositionLong
}
