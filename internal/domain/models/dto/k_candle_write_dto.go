package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// KCandleWriteDto is the shape the application hands the domain to create or update a K candle.
type KCandleWriteDto struct {
	Symbol              string
	OpenTime            time.Time
	Open                decimal.Decimal
	High                decimal.Decimal
	Low                 decimal.Decimal
	Close               decimal.Decimal
	Volume              decimal.Decimal
	QuoteVolume         decimal.Decimal
	TakerBuyBaseVolume  decimal.Decimal
	TakerBuyQuoteVolume decimal.Decimal
}
