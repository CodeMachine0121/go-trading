package vo

// PositionDirectionVo is which way a position is facing. Immutable, no behavior —
// what a direction earns or loses at a price lives in BacktestPositionDomain.
type PositionDirectionVo string

const (
	// PositionDirectionLong earns when the price rises and loses when it falls.
	PositionDirectionLong PositionDirectionVo = "long"
	// PositionDirectionShort earns when the price falls and loses when it rises.
	PositionDirectionShort PositionDirectionVo = "short"
)
