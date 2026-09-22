package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleContractWriteDto is the shape the application hands the domain to create or
// update a contract K candle.
//
// Every figure arrives as an optional even though a stored contract K candle has
// none. That is the point: "not given" has to be expressible for the domain to be
// the one place that rejects it. It reaches here missing for two quite different
// reasons — a caller who left it out, and a market source that answered one of the
// two series but not the other — and both deserve the same answer, which is only
// possible if they arrive in the same shape.
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
}
