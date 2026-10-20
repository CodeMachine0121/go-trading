package dto

import "github.com/shopspring/decimal"

// StrategyBotCapitalAllocationDto is one watched symbol's share of the bot's total capital, in percent.
type StrategyBotCapitalAllocationDto struct {
	Symbol               string
	AllocationPercentage decimal.Decimal
}
