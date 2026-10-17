package entities

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleContractSummary is one finished hour or day of contract candles merged ahead of time, in the same shape as a stored contract minute.
type KCandleContractSummary struct {
	ID       uint      `gorm:"primaryKey"`
	Symbol   string    `gorm:"size:64;not null;uniqueIndex:idx_k_candle_contract_summaries_symbol_interval_open_time,priority:1"`
	Interval string    `gorm:"size:8;not null;uniqueIndex:idx_k_candle_contract_summaries_symbol_interval_open_time,priority:2"`
	OpenTime time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_k_candle_contract_summaries_symbol_interval_open_time,priority:3"`

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

func (kCandleContractSummary KCandleContractSummary) TableName() string {
	return "KCandleContractSummaries"
}

func (kCandleContractSummary KCandleContractSummary) ToKCandleContract() KCandleContract {
	return KCandleContract{
		Symbol:              kCandleContractSummary.Symbol,
		OpenTime:            kCandleContractSummary.OpenTime.UTC(),
		Open:                kCandleContractSummary.Open,
		High:                kCandleContractSummary.High,
		Low:                 kCandleContractSummary.Low,
		Close:               kCandleContractSummary.Close,
		Volume:              kCandleContractSummary.Volume,
		QuoteVolume:         kCandleContractSummary.QuoteVolume,
		TakerBuyBaseVolume:  kCandleContractSummary.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleContractSummary.TakerBuyQuoteVolume,
		TradeCount:          kCandleContractSummary.TradeCount,
		MarkOpen:            kCandleContractSummary.MarkOpen,
		MarkHigh:            kCandleContractSummary.MarkHigh,
		MarkLow:             kCandleContractSummary.MarkLow,
		MarkClose:           kCandleContractSummary.MarkClose,
		IndexOpen:           kCandleContractSummary.IndexOpen,
		IndexHigh:           kCandleContractSummary.IndexHigh,
		IndexLow:            kCandleContractSummary.IndexLow,
		IndexClose:          kCandleContractSummary.IndexClose,
		PremiumIndexOpen:    kCandleContractSummary.PremiumIndexOpen,
		PremiumIndexHigh:    kCandleContractSummary.PremiumIndexHigh,
		PremiumIndexLow:     kCandleContractSummary.PremiumIndexLow,
		PremiumIndexClose:   kCandleContractSummary.PremiumIndexClose,
	}
}
