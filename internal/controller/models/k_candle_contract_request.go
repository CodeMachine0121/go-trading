package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleContractRequest is the body a caller sends to create or update a perpetual
// contract K candle. On update the candle is named by the path, so a symbol or open
// time in the body is only accepted when it matches.
//
// The mark, index and premium index prices and the trade count arrive as optionals so that leaving one out
// stays distinguishable from sending a zero — zero is a lawful trade count, and a
// blank mark price is the one thing that makes this not a contract candle at all.
// Both are refused by the domain rather than here, so that a caller's omission and a
// market source's gap are answered by the same rule.
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

// ToWriteDto turns the request into the shape the domain accepts, taking the identity
// from the arguments so the caller of this method decides what is named.
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
