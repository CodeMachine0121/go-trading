package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// SpotTradeStatisticsDto is always one group per market, Taiwan stock first, since their money cannot be added up.
type SpotTradeStatisticsDto struct {
	Period  string                         `json:"period"`
	Markets []SpotTradeMarketStatisticsDto `json:"markets"`
}

// SpotTradeMarketStatisticsDto rates are fractions and are null when there is nothing to rate.
type SpotTradeMarketStatisticsDto struct {
	Market            string          `json:"market"`
	Currency          string          `json:"currency"`
	ClosedTradeCount  int             `json:"closedTradeCount"`
	WinCount          int             `json:"winCount"`
	WinRate           *float64        `json:"winRate"`
	NetProfit         decimal.Decimal `json:"netProfit"`
	AverageReturnRate *float64        `json:"averageReturnRate"`
	// ProfitFactor is null when nothing lost.
	ProfitFactor *float64 `json:"profitFactor"`
	// AverageRMultiple counts only trades with a planned stop, RTradeCount of them.
	AverageRMultiple               *float64                            `json:"averageRMultiple"`
	RTradeCount                    int                                 `json:"rTradeCount"`
	CumulativeProfit               []SpotTradeCumulativeProfitPointDto `json:"cumulativeProfit"`
	ReturnDistribution             []SpotTradeReturnBucketDto          `json:"returnDistribution"`
	MistakeCosts                   []SpotTradeMistakeCostDto           `json:"mistakeCosts"`
	WithTradingStrategy            SpotTradeGroupStatisticsDto         `json:"withTradingStrategy"`
	SelfJudged                     SpotTradeGroupStatisticsDto         `json:"selfJudged"`
	AverageEntrySlippagePercentage *float64                            `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int                                 `json:"entrySlippageTradeCount"`
}

type SpotTradeCumulativeProfitPointDto struct {
	TradeID             uint            `json:"tradeId"`
	ClosedAt            time.Time       `json:"closedAt"`
	NetProfit           decimal.Decimal `json:"netProfit"`
	CumulativeNetProfit decimal.Decimal `json:"cumulativeNetProfit"`
}

type SpotTradeReturnBucketDto struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SpotTradeMistakeCostDto struct {
	TagID             uint            `json:"tagId"`
	Name              string          `json:"name"`
	TradeCount        int             `json:"tradeCount"`
	TotalNetProfit    decimal.Decimal `json:"totalNetProfit"`
	AverageReturnRate *float64        `json:"averageReturnRate"`
}

type SpotTradeGroupStatisticsDto struct {
	TradeCount        int      `json:"tradeCount"`
	WinRate           *float64 `json:"winRate"`
	AverageReturnRate *float64 `json:"averageReturnRate"`
}
