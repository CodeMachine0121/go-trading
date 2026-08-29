package dto

import "time"

// KCandleQueryDto is the shape the application hands the domain to query a time range.
type KCandleQueryDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
}
