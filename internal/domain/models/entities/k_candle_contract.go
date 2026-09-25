package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleContract is separate from KCandle and fully NOT NULL because its single source
// reports every figure; only the index price and premium index lines are nullable, for rows
// stored before those lines existed.
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

	IndexOpen  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	IndexHigh  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	IndexLow   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	IndexClose decimal.NullDecimal `gorm:"type:numeric(38,18)"`

	PremiumIndexOpen  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	PremiumIndexHigh  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	PremiumIndexLow   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	PremiumIndexClose decimal.NullDecimal `gorm:"type:numeric(38,18)"`
}

func (kCandleContract KCandleContract) TableName() string {
	return "KCandleContracts"
}

// ToDto returns the open time in UTC regardless of the zone it was read back in.
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
		IndexOpen:           kCandleContract.IndexOpen,
		IndexHigh:           kCandleContract.IndexHigh,
		IndexLow:            kCandleContract.IndexLow,
		IndexClose:          kCandleContract.IndexClose,
		PremiumIndexOpen:    kCandleContract.PremiumIndexOpen,
		PremiumIndexHigh:    kCandleContract.PremiumIndexHigh,
		PremiumIndexLow:     kCandleContract.PremiumIndexLow,
		PremiumIndexClose:   kCandleContract.PremiumIndexClose,
	}
}
