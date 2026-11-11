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

// TradingDirections are the directions this mode faces, long first; blank reads as long-short, the mode nothing said means.
func (contractTradingModeVo ContractTradingModeVo) TradingDirections() []TradingDirectionVo {
	switch contractTradingModeVo {
	case ContractTradingModeLongOnly:
		return []TradingDirectionVo{TradingDirectionLong}
	case ContractTradingModeShortOnly:
		return []TradingDirectionVo{TradingDirectionShort}
	default:
		return []TradingDirectionVo{TradingDirectionLong, TradingDirectionShort}
	}
}
