package vo

import "github.com/shopspring/decimal"

// ContractMarketOrderVo is one market order; ClientOrderID is fixed per step so a resend can be recognised as the same order.
type ContractMarketOrderVo struct {
	Symbol        string
	Side          ContractOrderSideVo
	Quantity      decimal.Decimal
	ReduceOnly    bool
	ClientOrderID string
}
