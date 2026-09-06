package vo

// PositionSizingModeVo is how much one opening stakes. Immutable, no behavior — the
// arithmetic and the range each mode allows live in PositionSizingDomain.
type PositionSizingModeVo string

const (
	// PositionSizingModeAllIn stakes every unit of cash on hand at the time, so it
	// carries no number of its own.
	PositionSizingModeAllIn PositionSizingModeVo = "allIn"
	// PositionSizingModePercentage stakes that percentage of the cash on hand.
	PositionSizingModePercentage PositionSizingModeVo = "percentage"
	// PositionSizingModeFixedAmount stakes the same figure every time, and skips the
	// opening entirely when the cash on hand cannot cover it.
	PositionSizingModeFixedAmount PositionSizingModeVo = "fixedAmount"
)
