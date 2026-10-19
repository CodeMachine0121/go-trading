package dto

import "github.com/shopspring/decimal"

// SymbolScreeningProfileWriteDto leaves optional figures nil so the profile's defaults fill them.
type SymbolScreeningProfileWriteDto struct {
	Name                           string
	StrategyScriptID               uint
	ParameterValues                []StrategyScriptParameterValueDto
	AggregationInterval            string
	TradingMode                    string
	Leverage                       *decimal.Decimal
	EntryCostPercentage            *decimal.Decimal
	SlippagePercentage             *decimal.Decimal
	HistoryDays                    *int
	SegmentCount                   *int
	MinimumTradeCount              *int
	MinimumAverageDailyQuoteVolume *decimal.Decimal
	BaselineSymbols                []string
}
