package vo

import "time"

// TradingSessionOccurrenceVo is one stretch of trading on one actual day: when it
// ran, and which business day its trading counts towards.
//
// It is the unit almost everything about a closing market turns out to want. "Is the
// market open" is whether one of these holds the moment; "has it had a chance to say
// anything" is how far into one we are; "we already decided this is shut" is a
// decision about one of these rather than about a date — which is what stops a shut
// day board from speaking for the evening board that follows it.
//
// The business date is carried rather than derived on demand, because deriving it
// needs the market's own trading calendar and this value travels to places that do
// not have one.
//
// Immutable, no behavior beyond reading itself.
type TradingSessionOccurrenceVo struct {
	// StartTime is when this stretch began, in universal time.
	StartTime time.Time
	// EndTime is when it ended, exclusive, in universal time.
	EndTime time.Time
	// BusinessDate is midnight of the business day this stretch's trading counts
	// towards, in the market's own zone, expressed universally.
	BusinessDate time.Time
}

// NewTradingSessionOccurrenceVo pins all three moments to universal time, whatever
// zone the caller worked in.
func NewTradingSessionOccurrenceVo(
	startTime time.Time, endTime time.Time, businessDate time.Time,
) TradingSessionOccurrenceVo {
	return TradingSessionOccurrenceVo{
		StartTime:    startTime.UTC(),
		EndTime:      endTime.UTC(),
		BusinessDate: businessDate.UTC(),
	}
}

// Contains reports a moment falling inside this stretch — the opening bell included,
// the closing one not, because a candle stamped at the close would cover time the
// market was already shut for.
func (tradingSessionOccurrenceVo TradingSessionOccurrenceVo) Contains(moment time.Time) bool {
	return !moment.Before(tradingSessionOccurrenceVo.StartTime) &&
		moment.Before(tradingSessionOccurrenceVo.EndTime)
}

// Length is how long this stretch ran for.
func (tradingSessionOccurrenceVo TradingSessionOccurrenceVo) Length() time.Duration {
	return tradingSessionOccurrenceVo.EndTime.Sub(tradingSessionOccurrenceVo.StartTime)
}

// IsZero reports the value that means "no stretch at all", which is what a market
// with no hours answers with and what a moment outside every stretch reads back.
func (tradingSessionOccurrenceVo TradingSessionOccurrenceVo) IsZero() bool {
	return tradingSessionOccurrenceVo.StartTime.IsZero()
}
