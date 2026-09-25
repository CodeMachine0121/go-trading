package vo

import "github.com/shopspring/decimal"

// ExitPricesVo is a position's exit prices, fixed at entry; each has its own presence flag because a 100% stop legitimately prices at zero.
type ExitPricesVo struct {
	// StopLossPrice sits below the entry because a spot position is only ever long.
	StopLossPrice   decimal.Decimal
	HasStopLoss     bool
	TakeProfitPrice decimal.Decimal
	HasTakeProfit   bool
}
