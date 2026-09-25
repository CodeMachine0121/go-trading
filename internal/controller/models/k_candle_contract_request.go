package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleContractRequest's body symbol and open time must match the path on update; the optional prices and trade count keep "left out" distinct from zero, and the domain refuses a missing one.
type KCandleContractRequest struct {
	Symbol              string              `json:"symbol"`
	OpenTime            time.Time           `json:"openTime"`
	Open                decimal.Decimal     `json:"open"`
	High                decimal.Decimal     `json:"high"`
	Low                 decimal.Decimal     `json:"low"`
	Close               decimal.Decimal     `json:"close"`
	Volume              decimal.Decimal     `json:"volume"`
	QuoteVolume         decimal.NullDecimal `json:"quoteVolume"`
	TakerBuyBaseVolume  decimal.NullDecimal `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.NullDecimal `json:"takerBuyQuoteVolume"`
	TradeCount          *int64              `json:"tradeCount"`
	MarkOpen            decimal.NullDecimal `json:"markOpen"`
	MarkHigh            decimal.NullDecimal `json:"markHigh"`
	MarkLow             decimal.NullDecimal `json:"markLow"`
	MarkClose           decimal.NullDecimal `json:"markClose"`
	IndexOpen           decimal.NullDecimal `json:"indexOpen"`
	IndexHigh           decimal.NullDecimal `json:"indexHigh"`
	IndexLow            decimal.NullDecimal `json:"indexLow"`
	IndexClose          decimal.NullDecimal `json:"indexClose"`
	PremiumIndexOpen    decimal.NullDecimal `json:"premiumIndexOpen"`
	PremiumIndexHigh    decimal.NullDecimal `json:"premiumIndexHigh"`
	PremiumIndexLow     decimal.NullDecimal `json:"premiumIndexLow"`
	PremiumIndexClose   decimal.NullDecimal `json:"premiumIndexClose"`
}

// ToWriteDto takes the candle identity from the arguments rather than the body.
func (kCandleContractRequest KCandleContractRequest) ToWriteDto(
	symbol string, openTime time.Time,
) dto.KCandleContractWriteDto {
	return dto.KCandleContractWriteDto{
		Symbol:              symbol,
		OpenTime:            openTime,
		Open:                kCandleContractRequest.Open,
		High:                kCandleContractRequest.High,
		Low:                 kCandleContractRequest.Low,
		Close:               kCandleContractRequest.Close,
		Volume:              kCandleContractRequest.Volume,
		QuoteVolume:         kCandleContractRequest.QuoteVolume,
		TakerBuyBaseVolume:  kCandleContractRequest.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleContractRequest.TakerBuyQuoteVolume,
		TradeCount:          kCandleContractRequest.TradeCount,
		MarkOpen:            kCandleContractRequest.MarkOpen,
		MarkHigh:            kCandleContractRequest.MarkHigh,
		MarkLow:             kCandleContractRequest.MarkLow,
		MarkClose:           kCandleContractRequest.MarkClose,
		IndexOpen:           kCandleContractRequest.IndexOpen,
		IndexHigh:           kCandleContractRequest.IndexHigh,
		IndexLow:            kCandleContractRequest.IndexLow,
		IndexClose:          kCandleContractRequest.IndexClose,
		PremiumIndexOpen:    kCandleContractRequest.PremiumIndexOpen,
		PremiumIndexHigh:    kCandleContractRequest.PremiumIndexHigh,
		PremiumIndexLow:     kCandleContractRequest.PremiumIndexLow,
		PremiumIndexClose:   kCandleContractRequest.PremiumIndexClose,
	}
}
