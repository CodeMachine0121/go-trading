package dto

import "github.com/shopspring/decimal"

// ContractOrderRefusalDto Reason is belowMinimumQuantity, belowMinimumNotional or
// aboveTierLeverage.
type ContractOrderRefusalDto struct {
	Reason              string
	Quantity            decimal.Decimal
	MinimumQuantity     decimal.Decimal
	Notional            decimal.Decimal
	MinimumNotional     decimal.Decimal
	TierMaximumLeverage int
}
