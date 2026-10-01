package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// BinanceTradingKeyVerificationDomain turns the exchange's answer into the markets to record, or the reason the key is refused.
type BinanceTradingKeyVerificationDomain struct {
	verification vo.TradingKeyVerificationVo
}

func NewBinanceTradingKeyVerificationDomain(
	verification vo.TradingKeyVerificationVo,
) BinanceTradingKeyVerificationDomain {
	return BinanceTradingKeyVerificationDomain{verification: verification}
}

// TradableMarkets refuses an accepted key with neither trading permission, since a read-only key can never place an order.
func (verificationDomain BinanceTradingKeyVerificationDomain) TradableMarkets() (TradableMarketsDomain, error) {
	if verificationDomain.verification.FailureReason != vo.TradingKeyVerificationFailureNone {
		return TradableMarketsDomain{}, BinanceTradingKeyVerificationError{
			Reason: verificationDomain.verification.FailureReason,
		}
	}

	tradableMarkets := NewTradableMarketsDomain(
		verificationDomain.verification.SpotTradingEnabled,
		verificationDomain.verification.ContractTradingEnabled)
	if tradableMarkets.IsEmpty() {
		return TradableMarketsDomain{}, BinanceTradingKeyVerificationError{
			Reason: vo.TradingKeyVerificationFailureNoTradingPermission,
		}
	}

	return tradableMarkets, nil
}
