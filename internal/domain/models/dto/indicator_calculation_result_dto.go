package dto

import "time"

// IndicatorCalculationResultDto reports the stretch it read so callers can map values onto
// candles without re-deriving the grid.
type IndicatorCalculationResultDto struct {
	Symbol string `json:"symbol"`
	// Interval is the one actually used, even when the caller named none.
	Interval string `json:"interval"`
	// RequiredCandleCount and UsedCandleCount differ only when the stretch is partly stored;
	// RequiredCandleCount includes the look-back, so it is not the request's candleCount.
	RequiredCandleCount int `json:"requiredCandleCount"`
	UsedCandleCount     int `json:"usedCandleCount"`
	// OpenTimes lists each candle's open time, earliest first; the nth list value belongs to
	// the nth open time.
	OpenTimes  []time.Time `json:"openTimes"`
	ResultType string      `json:"resultType"`
	// Values is keyed by indicator name and is empty under the signal kind.
	Values map[string]IndicatorValueDto `json:"values"`
	// Signal is buy, sell or hold under the signal kind and empty otherwise.
	Signal string `json:"signal,omitempty"`
}
