package dto

type BacktestRunSubmissionDto struct {
	Kind              string
	Request           BacktestRunRequestDto
	StrategyScriptID  *uint
	TradingStrategyID *uint
	SubjectName       string
}
