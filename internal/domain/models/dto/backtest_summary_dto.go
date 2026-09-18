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
	// StopLossExitCount and TakeProfitExitCount are how many finished trades ended
	// at one of the two exit levels the replay was given. Both are zero when it was
	// given none.
	//
	// They are here because the same return rate tells two entirely different
	// stories: a strategy where eight of ten exits were stops has been kept alive by
	// them, and one where all ten came from its own signals has not yet met the
	// stretch of market that empties it. Without these, those two read identically.
	StopLossExitCount   int `json:"stopLossExitCount"`
	TakeProfitExitCount int `json:"takeProfitExitCount"`
	// ConflictedCandleCount is how many candles had both conditions holding at once.
	// Those candles do nothing — picking a side would hand somebody an opinion the
	// system invented — and this is the only way they ever find out.
	//
	// It is a count rather than a flag because what matters is *how often*: once is a
	// coincidence, a hundred and eighty times out of two hundred means the trading
	// strategy is not deciding anything, and the report card of a replay that barely
	// traded reads as a very steady strategy indeed.
	//
	// Replaying a single strategy script always reports zero: one script does not
	// conflict with itself. Both kinds of replay hand back one shape, so nothing
	// reading a report card has to know which kind produced it.
	ConflictedCandleCount int `json:"conflictedCandleCount"`
}
