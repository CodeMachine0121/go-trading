package vo

// PositionDirectionVo is which way a position faces; spot replays only ever go long. See BacktestPositionDomain.
type PositionDirectionVo string

const (
	PositionDirectionLong PositionDirectionVo = "long"
	// PositionDirectionShort is only opened by contract replays.
	PositionDirectionShort PositionDirectionVo = "short"
)
