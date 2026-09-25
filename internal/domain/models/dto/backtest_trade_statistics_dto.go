package dto

import "github.com/shopspring/decimal"

// BacktestTradeStatisticsDto uses null rather than zero for figures that do not apply.
type BacktestTradeStatisticsDto struct {
	// ProfitFactor is null when nothing lost.
	ProfitFactor          *float64            `json:"profitFactor"`
	Expectancy            decimal.NullDecimal `json:"expectancy"`
	AverageHoldingSeconds *int64              `json:"averageHoldingSeconds"`
	// MaximumConsecutiveLossCount is ended by a break-even or winning trade.
	MaximumConsecutiveLossCount int `json:"maximumConsecutiveLossCount"`
	// CostToGrossProfitRatio is a fraction and is null when gross profit is zero.
	CostToGrossProfitRatio *float64 `json:"costToGrossProfitRatio"`
}
