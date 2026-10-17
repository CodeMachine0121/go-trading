package entities

import "time"

// KCandleContractSummaryDay is KCandleSummaryDay for contracts, kept apart because a contract and a spot market can share a symbol.
type KCandleContractSummaryDay struct {
	ID         uint      `gorm:"primaryKey"`
	Symbol     string    `gorm:"size:64;not null;uniqueIndex:idx_k_candle_contract_summary_days_symbol_day,priority:1"`
	Day        time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candle_contract_summary_days_symbol_day,priority:2"`
	Revision   int64     `gorm:"not null"`
	Summarized bool      `gorm:"not null"`

	Summaries []KCandleContractSummary `gorm:"-"`
}

func (kCandleContractSummaryDay KCandleContractSummaryDay) TableName() string {
	return "KCandleContractSummaryDays"
}
