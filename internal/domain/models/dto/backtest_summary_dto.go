package dto

import "github.com/shopspring/decimal"

type BacktestSummaryDto struct {
	BacktestTradeStatisticsDto
	InitialCapital decimal.Decimal `json:"initialCapital"`
	// FinalEquity includes the still-open position valued at the last candle's close.
	FinalEquity     decimal.Decimal `json:"finalEquity"`
	TotalReturnRate float64         `json:"totalReturnRate"`
	MaximumDrawdown float64         `json:"maximumDrawdown"`
	// WinRate is nil when nothing closed, so it is not confused with every trade losing.
	WinRate *float64 `json:"winRate"`
	// PositionOpenCount excludes openings skipped for lack of cash and includes the position
	// still open at the end.
	PositionOpenCount int `json:"positionOpenCount"`
	// StopLossExitCount and TakeProfitExitCount count trades closed at the given exit
	// levels; there is no liquidation count because spot replays never borrow.
	StopLossExitCount   int `json:"stopLossExitCount"`
	TakeProfitExitCount int `json:"takeProfitExitCount"`
	// TotalTransactionCost covers both charges on closed trades plus the entry charge of a
	// still-open position; it excludes that position's pending exit charge and any per-order
	// minimum fee.
	TotalTransactionCost decimal.Decimal `json:"totalTransactionCost"`
	// ConflictedCandleCount counts candles where both conditions held and nothing was done;
	// single-script replays always report zero.
	ConflictedCandleCount int `json:"conflictedCandleCount"`
}
