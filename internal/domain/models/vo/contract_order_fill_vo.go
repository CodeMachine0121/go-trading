package vo

import "github.com/shopspring/decimal"

// ContractOrderFillVo is how much of a market order was executed and at what average price.
type ContractOrderFillVo struct {
	ExecutedQuantity decimal.Decimal
	AveragePrice     decimal.Decimal
}
