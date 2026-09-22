package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleContractDto is the only shape in which a contract K candle leaves the
// domain. Nothing here is optional: a stored contract K candle is a complete one.
type KCandleContractDto struct {
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
	TradeCount          int64           `json:"tradeCount"`
	MarkOpen            decimal.Decimal `json:"markOpen"`
	MarkHigh            decimal.Decimal `json:"markHigh"`
	MarkLow             decimal.Decimal `json:"markLow"`
	MarkClose           decimal.Decimal `json:"markClose"`
}
