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
	TradingModeLeveragedLong TradingModeVo = "leveragedLong"
	// TradingModeShortOnly is the mirror of leveraged-long: it only ever goes short.
	// A sell opens a short, a buy closes back to cash, and a buy while flat does
	// nothing at all — there is nothing to close, and this mode cannot go long.
	//
	// It may borrow, and that is not a convenience: selling what you do not have
	// means borrowing it first, so there is no version of this mode that does not.
	// That is also why "leveraged" stays out of its name, where leveraged-long needs
	// it — borrowing is the only thing separating that one from spot, while here it
	// is not a choice anyone could have made differently.
	TradingModeShortOnly TradingModeVo = "shortOnly"
)
