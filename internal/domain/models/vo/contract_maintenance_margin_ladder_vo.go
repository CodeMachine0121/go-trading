package vo

import "github.com/shopspring/decimal"

// ContractMaintenanceMarginLadderVo is one contract's maintenance margin tiers as the venue reported them.
type ContractMaintenanceMarginLadderVo struct {
	Symbol string
	Tiers  []ContractMaintenanceMarginTierVo
}

type ContractMaintenanceMarginTierVo struct {
	Tier                  int
	NotionalFloor         decimal.Decimal
	NotionalCap           decimal.Decimal
	MaintenanceMarginRate decimal.Decimal
	MaintenanceAmount     decimal.Decimal
	MaximumLeverage       int
}
