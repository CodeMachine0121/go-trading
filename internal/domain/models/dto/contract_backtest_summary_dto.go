package dto

import "github.com/shopspring/decimal"

// ContractBacktestSummaryDto is a contract replay's report card: every figure a spot
// replay reports, plus what only a contract account has — liquidation, funding, the
// two directions apart, the openings the venue refused, and whose maintenance margin
// figures were used.
type ContractBacktestSummaryDto struct {
	InitialCapital      decimal.Decimal `json:"initialCapital"`
	FinalEquity         decimal.Decimal `json:"finalEquity"`
	TotalReturnRate     float64         `json:"totalReturnRate"`
	MaximumDrawdown     float64         `json:"maximumDrawdown"`
	WinRate             *float64        `json:"winRate"`
	PositionOpenCount   int             `json:"positionOpenCount"`
	StopLossExitCount   int             `json:"stopLossExitCount"`
	TakeProfitExitCount int             `json:"takeProfitExitCount"`
	// LiquidationExitCount is how many positions were closed by the venue.
	LiquidationExitCount int             `json:"liquidationExitCount"`
	TotalTransactionCost decimal.Decimal `json:"totalTransactionCost"`
	// TotalFundingFee is what the replay paid in funding net of what it received; a
	// negative figure is net income.
	TotalFundingFee decimal.Decimal `json:"totalFundingFee"`
	LongTradeCount  int             `json:"longTradeCount"`
	// LongWinRate and ShortWinRate are absent when that side never finished a trade.
	LongWinRate     *float64 `json:"longWinRate"`
	ShortTradeCount int      `json:"shortTradeCount"`
	ShortWinRate    *float64 `json:"shortWinRate"`
	// BlockedOpeningCount is how many openings the venue's rules refused: too few
	// units, too little notional, or more leverage than the position's tier allows.
	// Openings the cash could not afford are not among them.
	BlockedOpeningCount    int                       `json:"blockedOpeningCount"`
	ConflictedCandleCount  int                       `json:"conflictedCandleCount"`
	MaintenanceMarginBasis MaintenanceMarginBasisDto `json:"maintenanceMarginBasis"`
}
