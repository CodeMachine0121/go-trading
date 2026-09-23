package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractBacktestResultDto is everything one contract replay produced. Nothing of it
// is stored.
type ContractBacktestResultDto struct {
	Symbol          string                     `json:"symbol"`
	Interval        string                     `json:"interval"`
	TradingMode     string                     `json:"tradingMode"`
	Leverage        decimal.Decimal            `json:"leverage"`
	StartTime       time.Time                  `json:"startTime"`
	EndTime         time.Time                  `json:"endTime"`
	UsedCandleCount int                        `json:"usedCandleCount"`
	Summary         ContractBacktestSummaryDto `json:"summary"`
	ClosedTrades    []ContractClosedTradeDto   `json:"closedTrades"`
	EquityCurve     []EquityPointDto           `json:"equityCurve"`
}
