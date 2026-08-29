package dto

// IndicatorCalculationResultDto is the only shape an indicator calculation leaves
// the domain in. Values holds one number per indicator name; how many names appear
// is the script's decision, and an empty set is a valid result.
type IndicatorCalculationResultDto struct {
	Symbol          string             `json:"symbol"`
	UsedCandleCount int                `json:"usedCandleCount"`
	Values          map[string]float64 `json:"values"`
}
