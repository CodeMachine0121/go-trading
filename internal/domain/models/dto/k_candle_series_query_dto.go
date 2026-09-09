package dto

import "time"

// KCandleSeriesQueryDto is the shape the application hands the domain to query a
// time range as an aggregated series.
//
// How long one candle covers is asked in one of two ways, never both: name an
// Interval, or say how many candles you can display and let the market decide. Naming
// neither means one minute, which is the length a stored K candle already covers.
type KCandleSeriesQueryDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
	// Interval is the coarseness the caller declared, exactly as it was written.
	// Empty means none was declared.
	Interval string
	// DisplayableCandleCount is how many candles the caller can show at once, which
	// the market turns into a coarseness.
	//
	// It is a pointer where the other optional fields are zero values, and the
	// difference is the point: an unnamed end time is a moment nobody could have
	// meant, but **zero candles is a number somebody can say** — and saying it is
	// refused. Read through a zero value, that refusal could never happen, because
	// "displays nothing" and "did not say" would arrive looking identical.
	DisplayableCandleCount *int
}

// ToQueryDto drops the interval, leaving the plain time-range query underneath. The
// rules about naming a trading symbol and not ending before it starts belong to that
// query, so an aggregated ask hands them over rather than answering them twice.
func (kCandleSeriesQueryDto KCandleSeriesQueryDto) ToQueryDto() KCandleQueryDto {
	return KCandleQueryDto{
		Symbol:    kCandleSeriesQueryDto.Symbol,
		StartTime: kCandleSeriesQueryDto.StartTime,
		EndTime:   kCandleSeriesQueryDto.EndTime,
	}
}
