package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractPositionStatisticArchiveVo is one five-minute statistic from the venue's daily archive, which keeps long/short ratios rather than shares; any figure may be blank.
type ContractPositionStatisticArchiveVo struct {
	Symbol        string
	StatisticTime time.Time

	OpenInterest      decimal.NullDecimal
	OpenInterestValue decimal.NullDecimal

	AccountLongShortRatio           decimal.NullDecimal
	TopTraderPositionLongShortRatio decimal.NullDecimal
}
