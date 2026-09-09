package dto

import "time"

// IndicatorCalculationRequestDto is the shape the application hands the domain to
// run one indicator calculation.
//
// How coarse the K candles are and which stretch of market to read are here rather
// than on the strategy being run: they describe one run, so the same algorithm can
// be run at any coarseness over any stretch of market.
type IndicatorCalculationRequestDto struct {
	Symbol string
	// StartTime is where the stretch of market to compute over begins. How many
	// values come out of it is not this long divided by the coarseness: it is how
	// much of it the symbol's market is actually open for, which a venue that shuts
	// overnight makes very different from the two.
	StartTime time.Time
	Script    string
	// AggregationInterval is the coarseness the caller declared, exactly as it was
	// written. Reading it — including leaving it out — is the domain's job.
	AggregationInterval string
	// ResultType is the indicator value kind the caller declared, exactly as it was
	// written. Reading it — including leaving it out — is the domain's job.
	ResultType string
	// EndTime is where that stretch ends, and therefore also the moment to compute up
	// to — one moment, not two. The zero value means none was named, which the domain
	// reads as now; a pointer would only add a dereference, since the zero value is
	// not a moment anybody could have meant.
	EndTime time.Time
	// Parameters are the algorithm's knobs as the caller declared them. They arrive
	// with the run rather than being looked up, because what runs here is a script,
	// not a saved strategy — the script may never have been saved at all.
	Parameters []StrategyParameterWriteDto
	// ParameterValues are what those knobs are worth this time. Anything left out
	// keeps the value it was declared with.
	ParameterValues []StrategyParameterValueDto
}
