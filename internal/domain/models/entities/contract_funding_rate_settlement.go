package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlement is one funding rate settlement of a perpetual
// contract: the moment money changed hands between the long side and the short side,
// and at what rate. It is a plain data model: fields, persistence mapping and shape
// conversion only.
//
// It is a record of its own rather than a column on the contract K candle because it
// happens a few times a day, not every minute. Written onto every candle it would
// either leave all but a handful blank or repeat the latest rate on each one — and a
// replay reading the second could no longer tell which minute was the settlement.
type ContractFundingRateSettlement struct {
	ID     uint   `gorm:"primaryKey"`
	Symbol string `gorm:"size:64;not null;uniqueIndex:idx_contract_funding_rate_settlements_symbol_settlement_time,priority:1"`
	// SettlementTime is kept exactly as the venue stated it, to the millisecond. The
	// venue occasionally stamps one a millisecond past the hour, and rounding it
	// would invent a moment the venue never named.
	SettlementTime time.Time       `gorm:"type:timestamptz;not null;uniqueIndex:idx_contract_funding_rate_settlements_symbol_settlement_time,priority:2"`
	FundingRate    decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	// MarkPrice is the mark price at the moment of settlement. It is nullable because
	// the venue's earliest settlements did not record one, and "not recorded" is not
	// a price of zero.
	MarkPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
}

// TableName pins the table to ContractFundingRateSettlements instead of GORM's default.
func (contractFundingRateSettlement ContractFundingRateSettlement) TableName() string {
	return "ContractFundingRateSettlements"
}

// ToDto converts this record into the shape the domain hands outwards, in universal
// time whatever zone it was read back in.
func (contractFundingRateSettlement ContractFundingRateSettlement) ToDto() dto.ContractFundingRateSettlementDto {
	return dto.ContractFundingRateSettlementDto{
		Symbol:         contractFundingRateSettlement.Symbol,
		SettlementTime: contractFundingRateSettlement.SettlementTime.UTC(),
		FundingRate:    contractFundingRateSettlement.FundingRate,
		MarkPrice:      contractFundingRateSettlement.MarkPrice,
	}
}
