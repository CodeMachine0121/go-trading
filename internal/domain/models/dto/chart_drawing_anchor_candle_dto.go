package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ChartDrawingAnchorCandleDto is the candle an assistant anchor is pinned to, the same shape for spot and contract.
type ChartDrawingAnchorCandleDto struct {
	OpenTime time.Time
	Open     decimal.Decimal
	High     decimal.Decimal
	Low      decimal.Decimal
	Close    decimal.Decimal
}
