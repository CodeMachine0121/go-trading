package vo

// ContractTradingModeVo is which set of rules a contract replay trades by: what "buy"
// and "sell" mean on a contract account. Immutable, no behavior — what each mode makes
// of a signal lives in ContractTradingModeDomain.
type ContractTradingModeVo string

const (
	// ContractTradingModeLongShort opens either way and reverses on the same bar.
	ContractTradingModeLongShort ContractTradingModeVo = "longShort"
	// ContractTradingModeLongOnly only ever holds long positions: a sell closes one.
	ContractTradingModeLongOnly ContractTradingModeVo = "longOnly"
	// ContractTradingModeShortOnly only ever holds short positions: a buy closes one.
	ContractTradingModeShortOnly ContractTradingModeVo = "shortOnly"
)
