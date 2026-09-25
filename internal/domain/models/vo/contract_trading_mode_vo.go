package vo

// ContractTradingModeVo is what buy and sell mean on a contract account; see ContractTradingModeDomain.
type ContractTradingModeVo string

const (
	// ContractTradingModeLongShort opens either way and reverses on the same bar.
	ContractTradingModeLongShort ContractTradingModeVo = "longShort"
	// ContractTradingModeLongOnly holds only longs: a sell closes one.
	ContractTradingModeLongOnly ContractTradingModeVo = "longOnly"
	// ContractTradingModeShortOnly holds only shorts: a buy closes one.
	ContractTradingModeShortOnly ContractTradingModeVo = "shortOnly"
)
