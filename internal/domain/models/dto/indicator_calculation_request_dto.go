package dto

import "time"

// IndicatorCalculationRequestDto is the shape the application hands the domain to
// run one indicator calculation.
//
// How coarse the K candles are, how many of them and up to when are all here rather
// than on the strategy being run: they describe one run, so the same algorithm can
// be run at any coarseness over any stretch of market.
type IndicatorCalculationRequestDto struct {
	Symbol string
	// CandleCount is how many aggregated candles to compute from, counted after
	// aggregating — twenty-four of them is a day at one-hour buckets and two hours
	// at five-minute ones.
	CandleCount int
	Script      string
	// AggregationInterval is the coarseness the caller declared, exactly as it was
	// written. Reading it — including leaving it out — is the domain's job.
	AggregationInterval string
	// ResultType is the indicator value kind the caller declared, exactly as it was
	// written. Reading it — including leaving it out — is the domain's job.
	ResultType string
	// EndTime is the moment to compute up to. The zero value means none was named,
	// which the domain reads as now — a pointer would only add a dereference, since
	// the zero value is not a moment anybody could have meant.
	EndTime time.Time
}
