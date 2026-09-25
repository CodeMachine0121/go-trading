package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractPositionStatisticArchiveVo is one five-minute position statistic as the
// venue's daily archive records it.
//
// **The archive keeps ratios, not shares.** Where the live source says how much of
// each side is long and how much short, the archive only keeps the one number those
// two make together, so the shares are worked out from it — and working them out is a
// rule, which is why this type stops at the ratio and leaves the rest to the domain.
//
// Every figure can be absent: the archive is a file anybody can write a blank cell
// into, and "a statistic missing any one of its figures is not stored" is a rule the
// domain keeps, not this type.
type ContractPositionStatisticArchiveVo struct {
	Symbol        string
	StatisticTime time.Time

	OpenInterest      decimal.NullDecimal
	OpenInterestValue decimal.NullDecimal

	AccountLongShortRatio           decimal.NullDecimal
	TopTraderPositionLongShortRatio decimal.NullDecimal
}
