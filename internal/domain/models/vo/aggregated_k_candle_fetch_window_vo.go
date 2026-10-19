package vo

import "time"

// AggregatedKCandleFetchWindowVo asks a venue for its own coarse candles; Step is the interval's length so paging can move past the last bar.
type AggregatedKCandleFetchWindowVo struct {
	Symbol    string
	Interval  string
	Step      time.Duration
	StartTime time.Time
	EndTime   time.Time
}
