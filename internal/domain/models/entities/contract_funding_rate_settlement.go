package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlement is its own table rather than a candle column because
// settlements happen only a few times a day.
type ContractFundingRateSettlement struct {
	ID     uint   `gorm:"primaryKey"`
	Symbol string `gorm:"size:64;not null;uniqueIndex:idx_contract_funding_rate_settlements_symbol_settlement_time,priority:1"`
	// SettlementTime keeps the venue's millisecond precision, since the venue sometimes
	// stamps a millisecond past the hour.
	SettlementTime time.Time       `gorm:"type:timestamptz;not null;uniqueIndex:idx_contract_funding_rate_settlements_symbol_settlement_time,priority:2"`
	FundingRate    decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	// MarkPrice is nullable because the venue's earliest settlements recorded none.
	MarkPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
}

func (contractFundingRateSettlement ContractFundingRateSettlement) TableName() string {
	return "ContractFundingRateSettlements"
}

// ToDto returns times in UTC regardless of the zone they were read back in.
func (contractFundingRateSettlement ContractFundingRateSettlement) ToDto() dto.ContractFundingRateSettlementDto {
	return dto.ContractFundingRateSettlementDto{
		Symbol:         contractFundingRateSettlement.Symbol,
		SettlementTime: contractFundingRateSettlement.SettlementTime.UTC(),
		FundingRate:    contractFundingRateSettlement.FundingRate,
		MarkPrice:      contractFundingRateSettlement.MarkPrice,
	}
}
