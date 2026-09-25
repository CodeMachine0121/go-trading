package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// EquityPointDto values any open position at that candle's close.
type EquityPointDto struct {
	OpenTime time.Time       `json:"openTime"`
	Equity   decimal.Decimal `json:"equity"`
}
