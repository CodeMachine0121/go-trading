package vo

import "github.com/shopspring/decimal"

// ContractOrderStateVo is an order as the venue last reported it; AveragePrice means nothing while ExecutedQuantity is zero.
type ContractOrderStateVo struct {
	ExchangeOrderID  string
	Progress         ContractOrderProgressVo
	ExecutedQuantity decimal.Decimal
	AveragePrice     decimal.Decimal
}
