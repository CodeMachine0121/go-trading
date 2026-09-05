package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// EquityPointDto is one point of the equity curve: what everything on hand was worth
// once a candle had closed, with any open position valued at that candle's close.
type EquityPointDto struct {
	OpenTime time.Time       `json:"openTime"`
	Equity   decimal.Decimal `json:"equity"`
}
