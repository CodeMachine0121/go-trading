package dto

import "github.com/shopspring/decimal"

// BacktestSummaryDto is one replay's report card.
type BacktestSummaryDto struct {
	InitialCapital decimal.Decimal `json:"initialCapital"`
	// FinalEquity includes the position still open when the replay ended, valued at
	// the last candle's close. Leaving it out would report the account as though the
	// last bet had never been placed.
	FinalEquity     decimal.Decimal `json:"finalEquity"`
	TotalReturnRate float64         `json:"totalReturnRate"`
	MaximumDrawdown float64         `json:"maximumDrawdown"`
	// WinRate is absent when nothing was ever closed. Nothing closed and every trade
	// lost are two different statements, and reporting the first as a rate of zero
	// makes them look like one.
	WinRate *float64 `json:"winRate"`
	// PositionOpenCount counts the openings that actually happened. An opening
	// skipped for want of cash is not one of them, and the position still open at
	// the end is.
	PositionOpenCount int `json:"positionOpenCount"`
}
