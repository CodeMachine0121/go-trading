package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleRequest is the body a caller sends to create or update a K candle.
// On update the candle is named by the path, so a symbol or open time in the body
// is only accepted when it matches.
type KCandleRequest struct {
	Symbol              string          `json:"symbol"`
	OpenTime            time.Time       `json:"openTime"`
	Open                decimal.Decimal `json:"open"`
	High                decimal.Decimal `json:"high"`
	Low                 decimal.Decimal `json:"low"`
	Close               decimal.Decimal `json:"close"`
	Volume              decimal.Decimal `json:"volume"`
	QuoteVolume         decimal.Decimal `json:"quoteVolume"`
	TakerBuyBaseVolume  decimal.Decimal `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.Decimal `json:"takerBuyQuoteVolume"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking the
// identity from the arguments so the caller of this method decides what is named.
func (kCandleRequest KCandleRequest) ToWriteDto(symbol string, openTime time.Time) dto.KCandleWriteDto {
	return dto.KCandleWriteDto{
		Symbol:              symbol,
		OpenTime:            openTime,
		Open:                kCandleRequest.Open,
		High:                kCandleRequest.High,
		Low:                 kCandleRequest.Low,
		Close:               kCandleRequest.Close,
		Volume:              kCandleRequest.Volume,
		QuoteVolume:         kCandleRequest.QuoteVolume,
		TakerBuyBaseVolume:  kCandleRequest.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleRequest.TakerBuyQuoteVolume,
	}
}
