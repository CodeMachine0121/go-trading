package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradingSpecificationDto is how trading one perpetual contract looks, as the
// venue last said it, and when that was.
//
// The two rates are proportions: a maintenance margin rate of 0.025 is two and a half
// percent of the position.
type ContractTradingSpecificationDto struct {
	TickSize               decimal.Decimal `json:"tickSize"`
	QuantityStep           decimal.Decimal `json:"quantityStep"`
	MinimumQuantity        decimal.Decimal `json:"minimumQuantity"`
	MinimumNotional        decimal.Decimal `json:"minimumNotional"`
	MaintenanceMarginRate  decimal.Decimal `json:"maintenanceMarginRate"`
	LiquidationFeeRate     decimal.Decimal `json:"liquidationFeeRate"`
	FundingIntervalHours   int             `json:"fundingIntervalHours"`
	SpecificationUpdatedAt time.Time       `json:"specificationUpdatedAt"`
}
