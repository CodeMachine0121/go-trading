package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type SymbolScreeningProfileDto struct {
	ID                             uint                              `json:"id"`
	Name                           string                            `json:"name"`
	StrategyScriptID               uint                              `json:"strategyScriptId"`
	ParameterValues                []StrategyScriptParameterValueDto `json:"parameterValues"`
	AggregationInterval            string                            `json:"aggregationInterval"`
	TradingMode                    string                            `json:"tradingMode"`
	Leverage                       decimal.Decimal                   `json:"leverage"`
	EntryCostPercentage            decimal.Decimal                   `json:"entryCostPercentage"`
	SlippagePercentage             decimal.Decimal                   `json:"slippagePercentage"`
	HistoryDays                    int                               `json:"historyDays"`
	SegmentCount                   int                               `json:"segmentCount"`
	MinimumTradeCount              int                               `json:"minimumTradeCount"`
	MinimumAverageDailyQuoteVolume decimal.Decimal                   `json:"minimumAverageDailyQuoteVolume"`
	BaselineSymbols                []string                          `json:"baselineSymbols"`
	CreatedAt                      time.Time                         `json:"createdAt"`
	UpdatedAt                      time.Time                         `json:"updatedAt"`
}
