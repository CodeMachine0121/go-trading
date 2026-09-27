package dto

import "time"

// SpotTradeLiveComparisonDto sets a spot strategy's closed live trades beside a replay of the same stretch, one row per symbol.
type SpotTradeLiveComparisonDto struct {
	TradingStrategyID      uint   `json:"tradingStrategyId"`
	TradingStrategyName    string `json:"tradingStrategyName"`
	TradingStrategyDeleted bool   `json:"tradingStrategyDeleted"`
	// NoClosedTrades is true when nothing linked to the strategy has closed, so nothing was replayed.
	NoClosedTrades bool                            `json:"noClosedTrades"`
	Rows           []SpotTradeLiveComparisonRowDto `json:"rows"`
	// AverageEntrySlippagePercentage spans every symbol and is null when no trade started from a bot round.
	AverageEntrySlippagePercentage *float64 `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int      `json:"entrySlippageTradeCount"`
}

type SpotTradeLiveComparisonRowDto struct {
	Symbol    string                  `json:"symbol"`
	Market    string                  `json:"market"`
	StartTime time.Time               `json:"startTime"`
	EndTime   time.Time               `json:"endTime"`
	Live      SpotTradeLiveFiguresDto `json:"live"`
	// Backtest is null when the replay could not run; BacktestUnavailableReason says why.
	Backtest                  *SpotTradeComparisonFiguresDto `json:"backtest"`
	BacktestUnavailableReason string                         `json:"backtestUnavailableReason"`
}

// SpotTradeComparisonFiguresDto win rate is a fraction and is null when nothing closed.
type SpotTradeComparisonFiguresDto struct {
	ClosedTradeCount int      `json:"closedTradeCount"`
	WinRate          *float64 `json:"winRate"`
}

// SpotTradeLiveFiguresDto adds what only live trades have: how far buys slipped from the bot's reference price.
type SpotTradeLiveFiguresDto struct {
	SpotTradeComparisonFiguresDto
	AverageEntrySlippagePercentage *float64 `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int      `json:"entrySlippageTradeCount"`
}

// SpotTradeComparisonPlanDto is every symbol's group plus the slippage across all of them.
type SpotTradeComparisonPlanDto struct {
	Groups                         []SpotTradeComparisonGroupDto
	AverageEntrySlippagePercentage *float64
	EntrySlippageTradeCount        int
}

// SpotTradeComparisonGroupDto is one symbol's live trades with the replay that stands beside them.
type SpotTradeComparisonGroupDto struct {
	Symbol          string
	Market          string
	Live            SpotTradeLiveFiguresDto
	BacktestRequest TradingStrategyBacktestRequestDto
}

// SpotTradeBacktestAttemptDto is how one group's replay went; FailureReason is set when it did not run.
type SpotTradeBacktestAttemptDto struct {
	Result        BacktestResultDto
	FailureReason string
}
