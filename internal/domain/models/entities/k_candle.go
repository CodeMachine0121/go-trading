package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandle is a single K candle row. It is a plain data model: fields, persistence
// mapping and shape conversion only, no business rules.
type KCandle struct {
	ID                  uint            `gorm:"primaryKey"`
	Symbol              string          `gorm:"size:64;not null;uniqueIndex:idx_k_candles_symbol_open_time,priority:1"`
	OpenTime            time.Time       `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candles_symbol_open_time,priority:2"`
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

// ToDto converts this record into the shape the domain hands outwards. The open
// time is always handed out in universal time, whatever zone it was read back in.
func (kCandle KCandle) ToDto() dto.KCandleDto {
	return dto.KCandleDto{
		Symbol:              kCandle.Symbol,
		OpenTime:            kCandle.OpenTime.UTC(),
		Open:                kCandle.Open,
		High:                kCandle.High,
		Low:                 kCandle.Low,
		Close:               kCandle.Close,
		Volume:              kCandle.Volume,
		QuoteVolume:         kCandle.QuoteVolume,
		TakerBuyBaseVolume:  kCandle.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandle.TakerBuyQuoteVolume,
	}
}
