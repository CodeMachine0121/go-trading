package vo

import "github.com/shopspring/decimal"

// PriceExtremesVo is the highest and lowest traded price over a stretch; Has is false when no candle fell inside it.
type PriceExtremesVo struct {
	HighestPrice decimal.Decimal
	LowestPrice  decimal.Decimal
	Has          bool
}
