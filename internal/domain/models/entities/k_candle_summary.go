package entities

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleSummary is one finished hour or day of spot candles merged ahead of time, in the same shape as a stored minute so the series merge reads both alike.
type KCandleSummary struct {
	ID                  uint                `gorm:"primaryKey"`
	Symbol              string              `gorm:"size:64;not null;uniqueIndex:idx_k_candle_summaries_symbol_interval_open_time,priority:1"`
	Interval            string              `gorm:"size:8;not null;uniqueIndex:idx_k_candle_summaries_symbol_interval_open_time,priority:2"`
	OpenTime            time.Time           `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candle_summaries_symbol_interval_open_time,priority:3"`
	Open                decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	High                decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Low                 decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Close               decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Volume              decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	QuoteVolume         decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakerBuyBaseVolume  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakerBuyQuoteVolume decimal.NullDecimal `gorm:"type:numeric(38,18)"`
}

func (kCandleSummary KCandleSummary) TableName() string {
	return "KCandleSummaries"
}

func (kCandleSummary KCandleSummary) ToKCandle() KCandle {
	return KCandle{
		Symbol:              kCandleSummary.Symbol,
		OpenTime:            kCandleSummary.OpenTime.UTC(),
		Open:                kCandleSummary.Open,
		High:                kCandleSummary.High,
		Low:                 kCandleSummary.Low,
		Close:               kCandleSummary.Close,
		Volume:              kCandleSummary.Volume,
		QuoteVolume:         kCandleSummary.QuoteVolume,
		TakerBuyBaseVolume:  kCandleSummary.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleSummary.TakerBuyQuoteVolume,
	}
}
