package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractPositionStatistic is the venue's five-minute open interest and long/short ratio
// reading, stored separately from candles because it is five-minutely and the venue keeps
// only thirty days.
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

func (contractPositionStatistic ContractPositionStatistic) TableName() string {
	return "ContractPositionStatistics"
}

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
