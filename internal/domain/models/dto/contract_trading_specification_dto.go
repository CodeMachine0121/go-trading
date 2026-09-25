package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradingSpecificationDto rates are proportions (0.025 is 2.5%).
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
