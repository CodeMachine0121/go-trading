package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleDto is the only shape in which a K candle leaves the domain.
type KCandleDto struct {
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
