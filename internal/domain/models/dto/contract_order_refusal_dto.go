package dto

import "github.com/shopspring/decimal"

// ContractOrderRefusalDto is why the venue would not take a suggested order, with the
// figures that say so. Reason is one of belowMinimumQuantity, belowMinimumNotional or
// aboveTierLeverage.
type ContractOrderRefusalDto struct {
	Reason              string
	Quantity            decimal.Decimal
	MinimumQuantity     decimal.Decimal
	Notional            decimal.Decimal
	MinimumNotional     decimal.Decimal
	TierMaximumLeverage int
}
