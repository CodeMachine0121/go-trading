package entities

import (
	"time"

	"github.com/shopspring/decimal"
)

// SymbolScreeningKCandle is a venue-aggregated candle kept apart from the followed minute candles, stamped with when it was fetched so it can expire.
type SymbolScreeningKCandle struct {
	ID       uint      `gorm:"primaryKey"`
	Symbol   string    `gorm:"size:64;not null;uniqueIndex:idx_symbol_screening_k_candles_symbol_interval_open_time,priority:1"`
	Interval string    `gorm:"size:8;not null;uniqueIndex:idx_symbol_screening_k_candles_symbol_interval_open_time,priority:2"`
	OpenTime time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_symbol_screening_k_candles_symbol_interval_open_time,priority:3"`

	Open                decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	High                decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Low                 decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Close               decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Volume              decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	QuoteVolume         decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TakerBuyBaseVolume  decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TakerBuyQuoteVolume decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	TradeCount          int64           `gorm:"not null"`

	MarkOpen  decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MarkHigh  decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MarkLow   decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	MarkClose decimal.Decimal `gorm:"type:numeric(38,18);not null"`

	FetchedAt time.Time `gorm:"type:timestamptz;not null;index"`
}

func (symbolScreeningKCandle SymbolScreeningKCandle) TableName() string {
	return "SymbolScreeningKCandles"
}

func (symbolScreeningKCandle SymbolScreeningKCandle) ToKCandleContract() KCandleContract {
	return KCandleContract{
		Symbol:              symbolScreeningKCandle.Symbol,
		OpenTime:            symbolScreeningKCandle.OpenTime.UTC(),
		Open:                symbolScreeningKCandle.Open,
		High:                symbolScreeningKCandle.High,
		Low:                 symbolScreeningKCandle.Low,
		Close:               symbolScreeningKCandle.Close,
		Volume:              symbolScreeningKCandle.Volume,
		QuoteVolume:         symbolScreeningKCandle.QuoteVolume,
		TakerBuyBaseVolume:  symbolScreeningKCandle.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: symbolScreeningKCandle.TakerBuyQuoteVolume,
		TradeCount:          symbolScreeningKCandle.TradeCount,
		MarkOpen:            symbolScreeningKCandle.MarkOpen,
		MarkHigh:            symbolScreeningKCandle.MarkHigh,
		MarkLow:             symbolScreeningKCandle.MarkLow,
		MarkClose:           symbolScreeningKCandle.MarkClose,
	}
}
