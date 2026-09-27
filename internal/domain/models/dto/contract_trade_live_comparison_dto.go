package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradeLiveComparisonDto sets a strategy's closed live trades beside a replay of the same stretch, one row per symbol.
type ContractTradeLiveComparisonDto struct {
	TradingStrategyID      uint   `json:"tradingStrategyId"`
	TradingStrategyName    string `json:"tradingStrategyName"`
	TradingStrategyDeleted bool   `json:"tradingStrategyDeleted"`
	// NoClosedTrades is true when nothing linked to the strategy has closed, so nothing was replayed.
	NoClosedTrades bool                                `json:"noClosedTrades"`
	Rows           []ContractTradeLiveComparisonRowDto `json:"rows"`
	// AverageEntrySlippagePercentage spans every symbol and is null when no trade started from a bot round.
	AverageEntrySlippagePercentage *float64 `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int      `json:"entrySlippageTradeCount"`
}

type ContractTradeLiveComparisonRowDto struct {
	Symbol    string                      `json:"symbol"`
	StartTime time.Time                   `json:"startTime"`
	EndTime   time.Time                   `json:"endTime"`
	Leverage  decimal.Decimal             `json:"leverage"`
	Live      ContractTradeLiveFiguresDto `json:"live"`
	// Backtest is null when the replay could not run; BacktestUnavailableReason says why.
	Backtest                  *ContractTradeComparisonFiguresDto `json:"backtest"`
	BacktestUnavailableReason string                             `json:"backtestUnavailableReason"`
}

// ContractTradeComparisonFiguresDto rates are fractions and are null for a side that never closed a trade.
type ContractTradeComparisonFiguresDto struct {
	ClosedTradeCount int      `json:"closedTradeCount"`
	WinRate          *float64 `json:"winRate"`
	LongWinRate      *float64 `json:"longWinRate"`
	ShortWinRate     *float64 `json:"shortWinRate"`
}

// ContractTradeLiveFiguresDto adds what only live trades have: how far fills slipped from the bot's reference price.
type ContractTradeLiveFiguresDto struct {
	ContractTradeComparisonFiguresDto
	AverageEntrySlippagePercentage *float64 `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int      `json:"entrySlippageTradeCount"`
}

// ContractTradeComparisonPlanDto is every symbol's group plus the slippage across all of them.
type ContractTradeComparisonPlanDto struct {
	Groups                         []ContractTradeComparisonGroupDto
	AverageEntrySlippagePercentage *float64
	EntrySlippageTradeCount        int
}

// ContractTradeComparisonGroupDto is one symbol's live trades with the replay that stands beside them.
type ContractTradeComparisonGroupDto struct {
	Symbol          string
	Live            ContractTradeLiveFiguresDto
	BacktestRequest ContractTradingStrategyBacktestRequestDto
}

// ContractTradeBacktestAttemptDto is how one group's replay went; FailureReason is set when it did not run.
type ContractTradeBacktestAttemptDto struct {
	Result        ContractBacktestResultDto
	FailureReason string
}
