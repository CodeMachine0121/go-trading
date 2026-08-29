package entities

import "github.com/shopspring/decimal"

// KCandle is a single OHLCV candlestick row. It is a plain data model:
// fields and persistence mapping only, no business behavior.
type KCandle struct {
	ID                  uint            `gorm:"primaryKey"`
	Open                decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	High                decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Low                 decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Close               decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Volume              decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	QuoteVolume         decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TakerBuyBaseVolume  decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TakerBuyQuoteVolume decimal.Decimal `gorm:"type:numeric(38,18);not null"`
}

// TableName pins the table to KCandles instead of GORM's default k_candles.
func (kCandle KCandle) TableName() string {
	return "KCandles"
}
