package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type KCandleDto struct {
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
