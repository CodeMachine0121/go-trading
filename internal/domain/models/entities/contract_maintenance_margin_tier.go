package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractMaintenanceMarginTier ladders are replaced whole, so all tiers of one contract
// share a confirmation time.
type ContractMaintenanceMarginTier struct {
	ID     uint   `gorm:"primaryKey"`
	Symbol string `gorm:"size:64;not null;uniqueIndex:idx_contract_maintenance_margin_tiers_symbol_tier,priority:1"`
	Tier   int    `gorm:"not null;uniqueIndex:idx_contract_maintenance_margin_tiers_symbol_tier,priority:2"`

	NotionalFloor         decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	NotionalCap           decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MaintenanceMarginRate decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MaintenanceAmount     decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MaximumLeverage       int             `gorm:"not null"`
	ConfirmedAt           time.Time       `gorm:"type:timestamptz;not null"`
}

func (contractMaintenanceMarginTier ContractMaintenanceMarginTier) TableName() string {
	return "ContractMaintenanceMarginTiers"
}

func (contractMaintenanceMarginTier ContractMaintenanceMarginTier) ToDto() dto.ContractMaintenanceMarginTierDto {
	return dto.ContractMaintenanceMarginTierDto{
		Symbol:                contractMaintenanceMarginTier.Symbol,
		Tier:                  contractMaintenanceMarginTier.Tier,
		NotionalFloor:         contractMaintenanceMarginTier.NotionalFloor,
		NotionalCap:           contractMaintenanceMarginTier.NotionalCap,
		MaintenanceMarginRate: contractMaintenanceMarginTier.MaintenanceMarginRate,
		MaintenanceAmount:     contractMaintenanceMarginTier.MaintenanceAmount,
		MaximumLeverage:       contractMaintenanceMarginTier.MaximumLeverage,
		ConfirmedAt:           contractMaintenanceMarginTier.ConfirmedAt.UTC(),
	}
}
