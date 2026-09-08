package dto

import "time"

// IndicatorCalculationResultDto is the only shape an indicator calculation leaves
// the domain in. Values holds one value per indicator name; how many names appear is
// the script's decision, and an empty set is a valid result. ResultType names the
// kind those values are, so a reader never has to look back at what was requested.
//
// It also says how much market a full answer would have taken next to how much there
// was to work from, so a short stretch is visible rather than merely shorter.
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
	Interval string `json:"interval"`
	// RequiredCandleCount is how many finished buckets a full answer would have taken,
	// and UsedCandleCount is how many there actually were to work from. They differ
	// only when the stretch asked about is only partly stored, and they are answered
	// together so that a caller can say so without deriving the first number a second
	// time from the span and the look-back.
	//
	// It is deliberately *not* called candleCount, which is what the request calls the
	// span it wants values for. The two are different numbers — a request for 100
	// values with a look-back of 20 needs 119 buckets — and sharing one name would
	// invite a caller to compare them and report a discrepancy that is not one.
	RequiredCandleCount int `json:"requiredCandleCount"`
	UsedCandleCount     int `json:"usedCandleCount"`
	// OpenTimes is where each candle the script saw begins, earliest first. The nth
	// value of a list-shaped indicator belongs to the nth of these. It is answered
	// whatever the kind: it describes what was read, not what came out.
	OpenTimes  []time.Time `json:"openTimes"`
	ResultType string      `json:"resultType"`
	// Values holds one value per indicator name for the four map-shaped kinds. Under
	// the signal kind it is empty and Signal carries the result instead — a signal
	// has no name, so there is nothing to key it by.
	Values map[string]IndicatorValueDto `json:"values"`
	// Signal is the whole result under the signal kind: buy, sell or hold. Empty for
	// every other kind, and omitted from the response there.
	Signal string `json:"signal,omitempty"`
}
