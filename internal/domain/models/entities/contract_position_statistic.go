package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractPositionStatistic is the venue's five-minute reading of where a perpetual
// contract's positions stand: how much is open, and how the long and short sides are
// split — across every account, and across the accounts holding the most. It is a
// plain data model: fields, persistence mapping and shape conversion only.
//
// It is a record of its own rather than columns on the contract K candle because it
// is taken every five minutes and the venue keeps only the last thirty days of it.
// Written onto one-minute candles, four in five would be blank, and every candle
// older than thirty days would be blank for good.
type ContractPositionStatistic struct {
	ID            uint      `gorm:"primaryKey"`
	Symbol        string    `gorm:"size:64;not null;uniqueIndex:idx_contract_position_statistics_symbol_statistic_time,priority:1"`
	StatisticTime time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_contract_position_statistics_symbol_statistic_time,priority:2"`

	OpenInterest      decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	OpenInterestValue decimal.Decimal `gorm:"type:numeric(38,18);not null"`

	AccountLongShare      decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	AccountShortShare     decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	AccountLongShortRatio decimal.Decimal `gorm:"type:numeric(38,18);not null"`

	TopTraderPositionLongShare      decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TopTraderPositionShortShare     decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TopTraderPositionLongShortRatio decimal.Decimal `gorm:"type:numeric(38,18);not null"`
}

// TableName pins the table to ContractPositionStatistics instead of GORM's default.
func (contractPositionStatistic ContractPositionStatistic) TableName() string {
	return "ContractPositionStatistics"
}

// ToDto converts this record into the shape the domain hands outwards.
func (contractPositionStatistic ContractPositionStatistic) ToDto() dto.ContractPositionStatisticDto {
	return dto.ContractPositionStatisticDto{
		Symbol:                          contractPositionStatistic.Symbol,
		StatisticTime:                   contractPositionStatistic.StatisticTime.UTC(),
		OpenInterest:                    contractPositionStatistic.OpenInterest,
		OpenInterestValue:               contractPositionStatistic.OpenInterestValue,
		AccountLongShare:                contractPositionStatistic.AccountLongShare,
		AccountShortShare:               contractPositionStatistic.AccountShortShare,
		AccountLongShortRatio:           contractPositionStatistic.AccountLongShortRatio,
		TopTraderPositionLongShare:      contractPositionStatistic.TopTraderPositionLongShare,
		TopTraderPositionShortShare:     contractPositionStatistic.TopTraderPositionShortShare,
		TopTraderPositionLongShortRatio: contractPositionStatistic.TopTraderPositionLongShortRatio,
	}
}
