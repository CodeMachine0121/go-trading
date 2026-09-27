package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradeStatisticsDto counts closed trades only; R figures leave out trades without a planned stop and say how many.
type ContractTradeStatisticsDto struct {
	Period           string `json:"period"`
	ClosedTradeCount int    `json:"closedTradeCount"`
	WinCount         int    `json:"winCount"`
	// WinRate is a fraction and is null when nothing closed, which is not the same as losing everything.
	WinRate          *float64        `json:"winRate"`
	NetProfit        decimal.Decimal `json:"netProfit"`
	AverageRMultiple *float64        `json:"averageRMultiple"`
	// ProfitFactor is null when nothing lost.
	ProfitFactor                   *float64                           `json:"profitFactor"`
	RExcludedCount                 int                                `json:"rExcludedCount"`
	CumulativeR                    []ContractTradeCumulativeRPointDto `json:"cumulativeR"`
	RDistribution                  []ContractTradeRBucketDto          `json:"rDistribution"`
	MistakeCosts                   []ContractTradeMistakeCostDto      `json:"mistakeCosts"`
	WithTradingStrategy            ContractTradeGroupStatisticsDto    `json:"withTradingStrategy"`
	SelfJudged                     ContractTradeGroupStatisticsDto    `json:"selfJudged"`
	AverageEntrySlippagePercentage *float64                           `json:"averageEntrySlippagePercentage"`
	EntrySlippageTradeCount        int                                `json:"entrySlippageTradeCount"`
	// FeeToGrossProfitRatio is a fraction and is null when no trade made a profit.
	FeeToGrossProfitRatio *float64 `json:"feeToGrossProfitRatio"`
}

type ContractTradeCumulativeRPointDto struct {
	TradeID             uint      `json:"tradeId"`
	ClosedAt            time.Time `json:"closedAt"`
	RMultiple           float64   `json:"rMultiple"`
	CumulativeRMultiple float64   `json:"cumulativeRMultiple"`
}

type ContractTradeRBucketDto struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ContractTradeMistakeCostDto struct {
	TagID          uint    `json:"tagId"`
	Name           string  `json:"name"`
	TradeCount     int     `json:"tradeCount"`
	TotalRMultiple float64 `json:"totalRMultiple"`
}

type ContractTradeGroupStatisticsDto struct {
	TradeCount       int      `json:"tradeCount"`
	WinRate          *float64 `json:"winRate"`
	AverageRMultiple *float64 `json:"averageRMultiple"`
}
