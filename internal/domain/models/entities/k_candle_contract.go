package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleContract is a single perpetual contract K candle row. It is a plain data
// model: fields, persistence mapping and shape conversion only, no business rules.
//
// It is a separate record from KCandle rather than the same one with extra columns,
// because the mark price is not a figure the spot market declines to publish — it is
// a figure the spot market has no concept of. Nothing is borrowed and nothing is
// lent: the same symbol at the same open time exists on both sides, each in its own
// table, and neither can overwrite the other.
//
// Every column is NOT NULL, which KCandle's cannot be. KCandle leaves turnover and
// taker volumes nullable to accommodate a market that does not report them; this
// record has exactly one source and that source reports all of them, so "a stored
// candle is a complete candle" is a guarantee of the schema rather than something
// each reader has to remember to check.
type KCandleContract struct {
	ID       uint      `gorm:"primaryKey"`
	Symbol   string    `gorm:"size:64;not null;uniqueIndex:idx_k_candle_contracts_symbol_open_time,priority:1"`
	OpenTime time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candle_contracts_symbol_open_time,priority:2"`

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
}

// TableName pins the table to KCandleContracts instead of GORM's default.
func (kCandleContract KCandleContract) TableName() string {
	return "KCandleContracts"
}

// ToDto converts this record into the shape the domain hands outwards. The open time
// is always handed out in universal time, whatever zone it was read back in.
func (kCandleContract KCandleContract) ToDto() dto.KCandleContractDto {
	return dto.KCandleContractDto{
		Symbol:              kCandleContract.Symbol,
		OpenTime:            kCandleContract.OpenTime.UTC(),
		Open:                kCandleContract.Open,
		High:                kCandleContract.High,
		Low:                 kCandleContract.Low,
		Close:               kCandleContract.Close,
		Volume:              kCandleContract.Volume,
		QuoteVolume:         kCandleContract.QuoteVolume,
		TakerBuyBaseVolume:  kCandleContract.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleContract.TakerBuyQuoteVolume,
		TradeCount:          kCandleContract.TradeCount,
		MarkOpen:            kCandleContract.MarkOpen,
		MarkHigh:            kCandleContract.MarkHigh,
		MarkLow:             kCandleContract.MarkLow,
		MarkClose:           kCandleContract.MarkClose,
	}
}
