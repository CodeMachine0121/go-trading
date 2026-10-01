package vo

// TradingKeyVerificationFailureVo is why the exchange would not take a trading key; the four stay distinct because each calls for a different fix.
type TradingKeyVerificationFailureVo string

const (
	TradingKeyVerificationFailureNone                TradingKeyVerificationFailureVo = ""
	TradingKeyVerificationFailureKeyRejected         TradingKeyVerificationFailureVo = "keyRejected"
	TradingKeyVerificationFailureUnreachable         TradingKeyVerificationFailureVo = "unreachable"
	TradingKeyVerificationFailureTimedOut            TradingKeyVerificationFailureVo = "timedOut"
	TradingKeyVerificationFailureNoTradingPermission TradingKeyVerificationFailureVo = "noTradingPermission"
)
