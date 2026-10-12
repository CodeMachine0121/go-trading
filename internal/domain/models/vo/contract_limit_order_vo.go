package vo

import "github.com/shopspring/decimal"

// ContractLimitOrderVo is one limit order resting until cancelled; ClientOrderID is fixed per step so a resend can be recognised as the same order.
type ContractLimitOrderVo struct {
	Symbol        string
	Side          ContractOrderSideVo
	Quantity      decimal.Decimal
	Price         decimal.Decimal
	ClientOrderID string
}
