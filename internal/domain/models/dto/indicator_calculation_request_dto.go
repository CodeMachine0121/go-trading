package dto

// IndicatorCalculationRequestDto is the shape the application hands the domain to
// run one indicator calculation.
type IndicatorCalculationRequestDto struct {
	Symbol      string
	CandleCount int
	Script      string
}
