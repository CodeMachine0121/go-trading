package dto

import "time"

// KCandleSeriesQueryDto takes either an Interval or a DisplayableCandleCount, never both;
// neither means one minute.
type KCandleSeriesQueryDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
	// Interval is raw caller input; empty means none was declared.
	Interval string
	// DisplayableCandleCount is a pointer so an explicit zero, which is refused, is
	// distinguishable from not given.
	DisplayableCandleCount *int
}

// ToQueryDto drops the interval so the plain range query's validation rules apply unchanged.
func (kCandleSeriesQueryDto KCandleSeriesQueryDto) ToQueryDto() KCandleQueryDto {
	return KCandleQueryDto{
		Symbol:    kCandleSeriesQueryDto.Symbol,
		StartTime: kCandleSeriesQueryDto.StartTime,
		EndTime:   kCandleSeriesQueryDto.EndTime,
	}
}
