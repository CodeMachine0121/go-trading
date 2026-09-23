package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractMaintenanceMarginTierDto is one tier of a contract's maintenance margin
// ladder, and when the ladder it belongs to was confirmed.
//
// The margin a position in this tier has to keep is its notional times the rate,
// less the amount.
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

// ContractMaintenanceMarginRefreshReportDto is what one refresh of the ladders did:
// how many contracts got a new ladder, and which ladders were refused and why. A
// refused contract keeps the ladder it had.
type ContractMaintenanceMarginRefreshReportDto struct {
	RefreshedCount int                           `json:"refreshedCount"`
	RefusedLadders []RefusedMaintenanceLadderDto `json:"refusedLadders"`
}

// RefusedMaintenanceLadderDto is one contract whose reported ladder could not be one.
type RefusedMaintenanceLadderDto struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason"`
}
