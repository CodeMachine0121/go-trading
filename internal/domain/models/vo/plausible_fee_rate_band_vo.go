package vo

import "github.com/shopspring/decimal"

// PlausibleFeeRateBandVo is how far a fill's fee may stray, as percentages of the fill's value, before it points at a wrong unit.
type PlausibleFeeRateBandVo struct {
	LowestPercentage  decimal.Decimal
	HighestPercentage decimal.Decimal
}
