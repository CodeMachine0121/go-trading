package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type KCandleWriteDto struct {
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
}
