package vo

import "time"

// SymbolScreeningSegmentVo is one stretch replayed on its own; EndTime is exclusive.
type SymbolScreeningSegmentVo struct {
	StartTime time.Time
	EndTime   time.Time
}
