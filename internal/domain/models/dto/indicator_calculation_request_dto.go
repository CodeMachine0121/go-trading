package dto

// IndicatorCalculationRequestDto is the shape the application hands the domain to
// run one indicator calculation.
type IndicatorCalculationRequestDto struct {
	Symbol      string
	CandleCount int
	Script      string
	// ResultType is the indicator value kind the caller declared, exactly as it was
	// written. Reading it — including leaving it out — is the domain's job.
	ResultType string
}
