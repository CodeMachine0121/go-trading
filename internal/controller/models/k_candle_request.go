package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// KCandleRequest's body symbol and open time must match the path on update.
type KCandleRequest struct {
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
}

// ToWriteDto takes the candle identity from the arguments rather than the body.
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
