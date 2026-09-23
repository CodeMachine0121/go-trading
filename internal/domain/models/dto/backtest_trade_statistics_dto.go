package dto

import "github.com/shopspring/decimal"

// BacktestTradeStatisticsDto is what a replay's finished round trips say about a
// short-term strategy, embedded in both kinds of report card. A figure that does not
// apply is null rather than zero: no trades and every trade lost are different things.
type BacktestTradeStatisticsDto struct {
	// ProfitFactor is what the winning trades made over what the losing ones lost. It
	// does not apply when nothing lost — it is not infinite.
	ProfitFactor *float64 `json:"profitFactor"`
	// Expectancy is the net profit per finished round trip.
	Expectancy decimal.NullDecimal `json:"expectancy"`
	// AverageHoldingSeconds is how long a finished round trip was held on average.
	AverageHoldingSeconds *int64 `json:"averageHoldingSeconds"`
	// MaximumConsecutiveLossCount is the longest run of losing round trips; breaking
	// even or winning ends a run.
	MaximumConsecutiveLossCount int `json:"maximumConsecutiveLossCount"`
	// CostToGrossProfitRatio is the charges for trading over what the trades made
	// before them, as a fraction. It does not apply when they made nothing before them.
	CostToGrossProfitRatio *float64 `json:"costToGrossProfitRatio"`
}
