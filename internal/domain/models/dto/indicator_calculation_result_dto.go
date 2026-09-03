package dto

import "time"

// IndicatorCalculationResultDto is the only shape an indicator calculation leaves
// the domain in. Values holds one value per indicator name; how many names appear is
// the script's decision, and an empty set is a valid result. ResultType names the
// kind those values are, so a reader never has to look back at what was requested.
//
// It also says which stretch of market it read, and that is not a courtesy: a caller
// putting a list of values back onto a chart has to know which candle each one
// belongs to. Left to work it out, it would have to cut the same grid a second time
// from the interval, the count and the end time — and the day the two ways of
// cutting disagree, the values land one bucket out with nothing reported.
type IndicatorCalculationResultDto struct {
	Symbol string `json:"symbol"`
	// Interval is the coarseness actually used, so a caller that named none still
	// learns what it got.
	Interval        string `json:"interval"`
	UsedCandleCount int    `json:"usedCandleCount"`
	// OpenTimes is where each candle the script saw begins, earliest first. The nth
	// value of a list-shaped indicator belongs to the nth of these. It is answered
	// whatever the kind: it describes what was read, not what came out.
	OpenTimes  []time.Time                  `json:"openTimes"`
	ResultType string                       `json:"resultType"`
	Values     map[string]IndicatorValueDto `json:"values"`
}
