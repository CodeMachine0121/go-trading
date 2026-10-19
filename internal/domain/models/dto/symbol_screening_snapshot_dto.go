package dto

import "github.com/shopspring/decimal"

// SymbolScreeningSnapshotDto is the profile and its script as they were at submission, so later edits never change a run.
type SymbolScreeningSnapshotDto struct {
	ProfileName                    string                            `json:"profileName"`
	StrategyScriptID               uint                              `json:"strategyScriptId"`
	Script                         string                            `json:"script"`
	ResultType                     string                            `json:"resultType"`
	Parameters                     []StrategyScriptParameterWriteDto `json:"parameters"`
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
}
