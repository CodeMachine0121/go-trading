package dto

import "time"

// KCandleSeriesQueryDto is the shape the application hands the domain to query a
// time range as an aggregated series. Interval is what the caller declared; leaving
// it empty means five minutes, which is the length a stored K candle already covers.
type KCandleSeriesQueryDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
	Interval  string
}
