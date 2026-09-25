package dto

import "time"

type IndicatorCalculationRequestDto struct {
	Symbol string
	// StartTime begins the stretch; the value count depends on the symbol's open trading
	// hours, not just span divided by interval.
	StartTime time.Time
	Script    string
	// AggregationInterval is raw caller input; the domain parses it, including the empty case.
	AggregationInterval string
	// ResultType is raw caller input; the domain parses it, including the empty case.
	ResultType string
	// EndTime is also the moment computed up to; zero means now.
	EndTime time.Time
	// Parameters travel with the run because the script may never have been saved.
	Parameters []StrategyScriptParameterWriteDto
	// ParameterValues override declared defaults; omitted ones keep their declared value.
	ParameterValues []StrategyScriptParameterValueDto
}
