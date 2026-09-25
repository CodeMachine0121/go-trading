package dto

import "github.com/shopspring/decimal"

type ContractBacktestSummaryDto struct {
	BacktestTradeStatisticsDto
	InitialCapital       decimal.Decimal `json:"initialCapital"`
	FinalEquity          decimal.Decimal `json:"finalEquity"`
	TotalReturnRate      float64         `json:"totalReturnRate"`
	MaximumDrawdown      float64         `json:"maximumDrawdown"`
	WinRate              *float64        `json:"winRate"`
	PositionOpenCount    int             `json:"positionOpenCount"`
	StopLossExitCount    int             `json:"stopLossExitCount"`
	TakeProfitExitCount  int             `json:"takeProfitExitCount"`
	LiquidationExitCount int             `json:"liquidationExitCount"`
	TotalTransactionCost decimal.Decimal `json:"totalTransactionCost"`
	// TotalFundingFee is net paid; negative is net income.
	TotalFundingFee decimal.Decimal `json:"totalFundingFee"`
	LongTradeCount  int             `json:"longTradeCount"`
	// LongWinRate and ShortWinRate are nil when that side never finished a trade.
	LongWinRate     *float64 `json:"longWinRate"`
	ShortTradeCount int      `json:"shortTradeCount"`
	ShortWinRate    *float64 `json:"shortWinRate"`
	// BlockedOpeningCount counts openings refused by venue rules (minimum quantity, minimum
	// notional, tier leverage), not ones the cash could not afford.
	BlockedOpeningCount    int                       `json:"blockedOpeningCount"`
	ConflictedCandleCount  int                       `json:"conflictedCandleCount"`
	MaintenanceMarginBasis MaintenanceMarginBasisDto `json:"maintenanceMarginBasis"`
}
