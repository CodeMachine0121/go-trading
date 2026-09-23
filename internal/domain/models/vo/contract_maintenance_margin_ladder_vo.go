package vo

import "github.com/shopspring/decimal"

// ContractMaintenanceMarginLadderVo is one contract's whole maintenance margin ladder as
// the venue reported it, tier by tier. Nothing is judged here.
type ContractMaintenanceMarginLadderVo struct {
	Symbol string
	Tiers  []ContractMaintenanceMarginTierVo
}

// ContractMaintenanceMarginTierVo is one tier of a ladder: the stretch of position
// notional it covers, the maintenance margin rate inside it, the fixed amount taken
// off the margin it gives, and the most leverage a position there may carry.
type ContractMaintenanceMarginTierVo struct {
	Tier                  int
	NotionalFloor         decimal.Decimal
	NotionalCap           decimal.Decimal
	MaintenanceMarginRate decimal.Decimal
	MaintenanceAmount     decimal.Decimal
	MaximumLeverage       int
}
