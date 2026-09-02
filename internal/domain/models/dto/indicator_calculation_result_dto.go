package dto

// IndicatorCalculationResultDto is the only shape an indicator calculation leaves
// the domain in. Values holds one value per indicator name; how many names appear is
// the script's decision, and an empty set is a valid result. ResultType names the
// kind those values are, so a reader never has to look back at what was requested.
type IndicatorCalculationResultDto struct {
	Symbol          string                       `json:"symbol"`
	UsedCandleCount int                          `json:"usedCandleCount"`
	ResultType      string                       `json:"resultType"`
	Values          map[string]IndicatorValueDto `json:"values"`
}
