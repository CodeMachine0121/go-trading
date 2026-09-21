package vo

// TradingModeVo is which set of rules a replay trades by. Immutable, no behavior —
// what each mode asks of a position lives in TradingModeDomain.
type TradingModeVo string

const (
	// TradingModeLongShort is always in the market: a sell reverses a long straight
	// into a short at the same price, so after the first opening there is no longer
	// such a thing as holding cash. It is the default.
	TradingModeLongShort TradingModeVo = "longShort"
	// TradingModeSpot only ever goes long: a sell closes back to cash and a sell
	// while flat does nothing at all, because there is nothing to sell and this mode
	// cannot short. It is also cash for goods, so it cannot borrow.
	TradingModeSpot TradingModeVo = "spot"
	// TradingModeLeveragedLong holds a position exactly as spot does — it only ever
	// goes long, a sell closes back to cash, and a sell while flat does nothing —
	// but it may hold a position worth more than the money behind it.
	//
	// It is the third of the three combinations that exist. Whether a mode may short
	// and whether it may borrow are separate questions, and only shorting-without-
	// borrowing is impossible: selling what you do not have means borrowing it first.
	TradingModeLeveragedLong TradingModeVo = "leveragedLong"
)
