package entities

import "time"

// KCandleSummaryDay is one spot symbol's UTC day of summaries; Revision moves on every minute write so a merge read before that write is never stored.
type KCandleSummaryDay struct {
	ID         uint      `gorm:"primaryKey"`
	Symbol     string    `gorm:"size:64;not null;uniqueIndex:idx_k_candle_summary_days_symbol_day,priority:1"`
	Day        time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candle_summary_days_symbol_day,priority:2"`
	Revision   int64     `gorm:"not null"`
	Summarized bool      `gorm:"not null"`

	Summaries []KCandleSummary `gorm:"-"`
}

func (kCandleSummaryDay KCandleSummaryDay) TableName() string {
	return "KCandleSummaryDays"
}
