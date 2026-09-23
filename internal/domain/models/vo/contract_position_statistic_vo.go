package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractPositionStatisticVo is one five-minute position statistic as the venue
// reported it, its three answers already aligned on the statistic time.
//
// The open interest decides which moments exist at all, so its two figures are
// always there. The two long-short splits come from answers of their own that may
// not cover the same moment, so each arrives absent when it did not — and the domain,
// not this type, decides that a statistic missing one is not stored.
type ContractPositionStatisticVo struct {
	Symbol            string
	StatisticTime     time.Time
	OpenInterest      decimal.Decimal
	OpenInterestValue decimal.Decimal

	AccountLongShare      decimal.NullDecimal
	AccountShortShare     decimal.NullDecimal
	AccountLongShortRatio decimal.NullDecimal

	TopTraderPositionLongShare      decimal.NullDecimal
	TopTraderPositionShortShare     decimal.NullDecimal
	TopTraderPositionLongShortRatio decimal.NullDecimal
}
