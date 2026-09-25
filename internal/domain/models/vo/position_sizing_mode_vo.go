package vo

// PositionSizingModeVo is how much one opening stakes; see PositionSizingDomain.
type PositionSizingModeVo string

const (
	// PositionSizingModeAllIn stakes all cash on hand, so it carries no value.
	PositionSizingModeAllIn      PositionSizingModeVo = "allIn"
	PositionSizingModePercentage PositionSizingModeVo = "percentage"
	// PositionSizingModeFixedAmount skips the opening when cash cannot cover the amount.
	PositionSizingModeFixedAmount PositionSizingModeVo = "fixedAmount"
)
