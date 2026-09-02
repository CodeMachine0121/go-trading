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
