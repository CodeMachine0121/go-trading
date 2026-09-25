package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractPositionStatisticVo is one five-minute statistic aligned on its time; open interest is always present, while each long/short split may be absent.
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
