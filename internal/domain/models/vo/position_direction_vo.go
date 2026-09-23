package vo

// PositionDirectionVo is which way a position is facing. Immutable, no behavior —
// what a direction earns or loses at a price lives in BacktestPositionDomain.
//
// **A spot replay only ever faces one way**: cash for goods, and the goods are only
// worth more when the price rises. A contract replay borrows, so it can also face the
// other way — which is why the set has two members and a spot replay never produces
// the second one.
type PositionDirectionVo string

const (
	// PositionDirectionLong earns when the price rises and loses when it falls.
	PositionDirectionLong PositionDirectionVo = "long"
	// PositionDirectionShort earns when the price falls and loses when it rises. Only a
	// contract replay opens one.
	PositionDirectionShort PositionDirectionVo = "short"
)
