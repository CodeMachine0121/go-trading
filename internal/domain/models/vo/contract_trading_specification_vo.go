package vo

import "github.com/shopspring/decimal"

// ContractTradingSpecificationVo is one contract's trading specification as the venue
// reported it, already normalized: the two rates are proportions, whatever unit the
// venue wrote them in.
//
// The funding interval is absent when the venue did not list the contract among those
// with a setting of their own. What an absent one means is a rule, and it is the
// domain's to apply.
type ContractTradingSpecificationVo struct {
	Symbol                string
	TickSize              decimal.Decimal
	QuantityStep          decimal.Decimal
	MinimumQuantity       decimal.Decimal
	MinimumNotional       decimal.Decimal
	MaintenanceMarginRate decimal.Decimal
	LiquidationFeeRate    decimal.Decimal
	FundingIntervalHours  *int
}
