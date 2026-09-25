package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleContractWriteDto uses optional figures so the domain alone rejects missing ones,
// whether omitted by a caller or missing from a partial market source response.
type KCandleContractWriteDto struct {
	Symbol              string
	OpenTime            time.Time
	Open                decimal.Decimal
	High                decimal.Decimal
	Low                 decimal.Decimal
	Close               decimal.Decimal
	Volume              decimal.Decimal
	QuoteVolume         decimal.NullDecimal
	TakerBuyBaseVolume  decimal.NullDecimal
	TakerBuyQuoteVolume decimal.NullDecimal
	TradeCount          *int64
	MarkOpen            decimal.NullDecimal
	MarkHigh            decimal.NullDecimal
	MarkLow             decimal.NullDecimal
	MarkClose           decimal.NullDecimal
	IndexOpen           decimal.NullDecimal
	IndexHigh           decimal.NullDecimal
	IndexLow            decimal.NullDecimal
	IndexClose          decimal.NullDecimal
	PremiumIndexOpen    decimal.NullDecimal
	PremiumIndexHigh    decimal.NullDecimal
	PremiumIndexLow     decimal.NullDecimal
	PremiumIndexClose   decimal.NullDecimal
}
