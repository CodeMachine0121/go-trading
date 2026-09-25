package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractTradingSymbol is separate from TradingSymbol because TradingSymbol keys on name
// alone, and it has no display name or market since it serves one venue whose pairs name
// themselves.
type ContractTradingSymbol struct {
	Symbol    string `gorm:"primaryKey;size:64;not null"`
	IsWatched bool   `gorm:"not null;default:false"`

	// Specification figures are nullable because contracts registered before specifications
	// were recorded have none until the first refresh; SpecificationUpdatedAt marks them as
	// set.
	TickSize               decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	QuantityStep           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MinimumQuantity        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MinimumNotional        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MaintenanceMarginRate  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	LiquidationFeeRate     decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	FundingIntervalHours   *int
	SpecificationUpdatedAt *time.Time `gorm:"type:timestamptz"`
}

func (contractTradingSymbol ContractTradingSymbol) TableName() string {
	return "ContractTradingSymbols"
}

func (contractTradingSymbol ContractTradingSymbol) ToDto() dto.ContractTradingSymbolDto {
	contractTradingSymbolDto := dto.ContractTradingSymbolDto{
		Symbol:    contractTradingSymbol.Symbol,
		IsWatched: contractTradingSymbol.IsWatched,
	}

	if contractTradingSymbol.SpecificationUpdatedAt != nil && contractTradingSymbol.FundingIntervalHours != nil {
		contractTradingSymbolDto.TradingSpecification = &dto.ContractTradingSpecificationDto{
			TickSize:               contractTradingSymbol.TickSize.Decimal,
			QuantityStep:           contractTradingSymbol.QuantityStep.Decimal,
			MinimumQuantity:        contractTradingSymbol.MinimumQuantity.Decimal,
			MinimumNotional:        contractTradingSymbol.MinimumNotional.Decimal,
			MaintenanceMarginRate:  contractTradingSymbol.MaintenanceMarginRate.Decimal,
			LiquidationFeeRate:     contractTradingSymbol.LiquidationFeeRate.Decimal,
			FundingIntervalHours:   *contractTradingSymbol.FundingIntervalHours,
			SpecificationUpdatedAt: contractTradingSymbol.SpecificationUpdatedAt.UTC(),
		}
	}

	return contractTradingSymbolDto
}
