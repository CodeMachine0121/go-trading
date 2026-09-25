package vo

// TargetPositionVo is what a candle's opinion asks the account to hold afterwards; see SignalDomain.TargetPosition and BacktestAccountDomain.
type TargetPositionVo string

const (
	TargetPositionLong TargetPositionVo = "long"
	// TargetPositionShort is only requested by contract trading modes (see ContractTradingModeDomain.TargetFor).
	TargetPositionShort TargetPositionVo = "short"
	// TargetPositionFlat asks to close everything, unlike TargetPositionUnchanged, which has no opinion.
	TargetPositionFlat TargetPositionVo = "flat"
	// TargetPositionUnchanged keeps whatever is held.
	TargetPositionUnchanged TargetPositionVo = "unchanged"
)

// WantsPosition answers for the spot account only; contract accounts check direction instead (see ContractBacktestAccountDomain.Apply).
func (targetPositionVo TargetPositionVo) WantsPosition() bool {
	return targetPositionVo == TargetPositionLong
}
