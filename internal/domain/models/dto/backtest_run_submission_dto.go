package dto

// BacktestRunSubmissionDto is what the application hands over once a request is resolved and validated: which replay, the request itself, and where it came from.
type BacktestRunSubmissionDto struct {
	Kind              string
	Request           BacktestRunRequestDto
	StrategyScriptID  *uint
	TradingStrategyID *uint
	SubjectName       string
}
