package vo

import "github.com/shopspring/decimal"

// ContractProtectiveOrderVo closes Quantity at market once the mark price reaches TriggerPrice, and never opens the other way.
type ContractProtectiveOrderVo struct {
	Symbol        string
	Side          ContractOrderSideVo
	Kind          ContractProtectiveOrderKindVo
	TriggerPrice  decimal.Decimal
	Quantity      decimal.Decimal
	ClientOrderID string
}
