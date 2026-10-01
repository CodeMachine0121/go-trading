package vo

// TradingKeyVerificationVo is the exchange's answer about a trading key; the permissions mean nothing unless FailureReason is none.
type TradingKeyVerificationVo struct {
	FailureReason          TradingKeyVerificationFailureVo
	SpotTradingEnabled     bool
	ContractTradingEnabled bool
}
