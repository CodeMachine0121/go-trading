package dto

import "github.com/shopspring/decimal"

// BacktestSummaryDto is one replay's report card.
type BacktestSummaryDto struct {
	// BacktestTradeStatisticsDto is what the finished round trips say about a
	// short-term strategy; its figures sit beside the others on the report card.
	BacktestTradeStatisticsDto
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
	//
	// There are two of them and not three. A replay borrows nothing, so no position
	// is ever taken off because the money behind it ran out — a count of that would
	// read zero on every report card this system can produce, which is a column
	// asking a question nobody here can answer.
	StopLossExitCount   int `json:"stopLossExitCount"`
	TakeProfitExitCount int `json:"takeProfitExitCount"`
	// TotalTransactionCost is everything this replay paid for the act of trading:
	// both charges on every finished round trip, plus the entry charge already paid
	// on a position still open at the end. It is zero when no rates were given.
	//
	// It is on the report card because one return rate tells two different stories: a
	// strategy that cannot read the market, and one that reads it well enough but
	// hands the winnings to the broker. Without this figure those two look identical,
	// and only one of them is worth another afternoon.
	//
	// Two things it deliberately does not do. It does not include the charge a still
	// open position would pay on the way out — that money has not moved, so the final
	// equity beside it is one exit charge too kind. And it knows nothing of a
	// per-order minimum fee, so very small orders are charged less here than a real
	// broker would charge.
	TotalTransactionCost decimal.Decimal `json:"totalTransactionCost"`
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
