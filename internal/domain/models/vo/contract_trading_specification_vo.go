package vo

import "github.com/shopspring/decimal"

// ContractTradingSpecificationVo is one contract's normalized specification, rates as proportions; an absent funding interval is interpreted by the domain.
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
