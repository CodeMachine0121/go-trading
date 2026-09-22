package vo

// PositionDirectionVo is which way a position is facing. Immutable, no behavior —
// what a direction earns or loses at a price lives in BacktestPositionDomain.
//
// **There is one direction.** A replay only ever trades spot: cash for goods, and the
// goods are only worth more when the price rises. Nothing here can open a position
// that gains when it falls.
//
// It stays a named value rather than becoming an absence, because every finished trade
// still says which way it faced and whoever reads that list is entitled to the answer.
// A replay of contracts would add the other one; this is a set with one member today,
// not a field with nothing in it.
type PositionDirectionVo string

const (
	// PositionDirectionLong earns when the price rises and loses when it falls.
	PositionDirectionLong PositionDirectionVo = "long"
)
