package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractMaintenanceMarginTierDto requires a position in this tier to keep notional × rate
// − amount as margin.
type ContractMaintenanceMarginTierDto struct {
	Symbol                string          `json:"symbol"`
	Tier                  int             `json:"tier"`
	NotionalFloor         decimal.Decimal `json:"notionalFloor"`
	NotionalCap           decimal.Decimal `json:"notionalCap"`
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
	MaintenanceAmount     decimal.Decimal `json:"maintenanceAmount"`
	MaximumLeverage       int             `json:"maximumLeverage"`
	ConfirmedAt           time.Time       `json:"confirmedAt"`
}

// ContractMaintenanceMarginRefreshReportDto lists refused ladders; a refused contract keeps
// its previous ladder.
type ContractMaintenanceMarginRefreshReportDto struct {
	RefreshedCount int                           `json:"refreshedCount"`
	RefusedLadders []RefusedMaintenanceLadderDto `json:"refusedLadders"`
}

type RefusedMaintenanceLadderDto struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason"`
}
