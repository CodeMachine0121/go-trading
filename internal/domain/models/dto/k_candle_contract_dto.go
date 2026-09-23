package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleContractDto is the only shape in which a contract K candle leaves the
// domain. Nothing here is optional except the index price and premium index lines,
// which are null only on a candle stored before either line existed — "not recorded
// when it was stored", never zero.
type KCandleContractDto struct {
	Symbol              string              `json:"symbol"`
	OpenTime            time.Time           `json:"openTime"`
	Open                decimal.Decimal     `json:"open"`
	High                decimal.Decimal     `json:"high"`
	Low                 decimal.Decimal     `json:"low"`
	Close               decimal.Decimal     `json:"close"`
	Volume              decimal.Decimal     `json:"volume"`
	QuoteVolume         decimal.Decimal     `json:"quoteVolume"`
	TakerBuyBaseVolume  decimal.Decimal     `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.Decimal     `json:"takerBuyQuoteVolume"`
	TradeCount          int64               `json:"tradeCount"`
	MarkOpen            decimal.Decimal     `json:"markOpen"`
	MarkHigh            decimal.Decimal     `json:"markHigh"`
	MarkLow             decimal.Decimal     `json:"markLow"`
	MarkClose           decimal.Decimal     `json:"markClose"`
	IndexOpen           decimal.NullDecimal `json:"indexOpen"`
	IndexHigh           decimal.NullDecimal `json:"indexHigh"`
	IndexLow            decimal.NullDecimal `json:"indexLow"`
	IndexClose          decimal.NullDecimal `json:"indexClose"`
	PremiumIndexOpen    decimal.NullDecimal `json:"premiumIndexOpen"`
	PremiumIndexHigh    decimal.NullDecimal `json:"premiumIndexHigh"`
	PremiumIndexLow     decimal.NullDecimal `json:"premiumIndexLow"`
	PremiumIndexClose   decimal.NullDecimal `json:"premiumIndexClose"`
}
