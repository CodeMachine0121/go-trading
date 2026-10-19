package entities

import (
	"time"

	"github.com/shopspring/decimal"
)

type SymbolScreeningFundingRateSettlement struct {
	ID             uint                `gorm:"primaryKey"`
	Symbol         string              `gorm:"size:64;not null;uniqueIndex:idx_symbol_screening_funding_rate_settlements_symbol_time,priority:1"`
	SettlementTime time.Time           `gorm:"type:timestamptz;not null;uniqueIndex:idx_symbol_screening_funding_rate_settlements_symbol_time,priority:2"`
	FundingRate    decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	MarkPrice      decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	FetchedAt      time.Time           `gorm:"type:timestamptz;not null;index"`
}

func (symbolScreeningFundingRateSettlement SymbolScreeningFundingRateSettlement) TableName() string {
	return "SymbolScreeningFundingRateSettlements"
}

func (symbolScreeningFundingRateSettlement SymbolScreeningFundingRateSettlement) ToContractFundingRateSettlement() ContractFundingRateSettlement {
	return ContractFundingRateSettlement{
		Symbol:         symbolScreeningFundingRateSettlement.Symbol,
		SettlementTime: symbolScreeningFundingRateSettlement.SettlementTime.UTC(),
		FundingRate:    symbolScreeningFundingRateSettlement.FundingRate,
		MarkPrice:      symbolScreeningFundingRateSettlement.MarkPrice,
	}
}
