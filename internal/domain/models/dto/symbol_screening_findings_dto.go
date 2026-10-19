package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// SymbolScreeningFindingsDto carries the figures behind a verdict; fields a candidate never reached stay nil.
type SymbolScreeningFindingsDto struct {
	HistoryDays              *int                               `json:"historyDays,omitempty"`
	AverageDailyQuoteVolume  *decimal.Decimal                   `json:"averageDailyQuoteVolume,omitempty"`
	Segments                 []SymbolScreeningSegmentFindingDto `json:"segments,omitempty"`
	ReturnWithoutBestSegment *float64                           `json:"returnWithoutBestSegment,omitempty"`
	Contribution             *SymbolScreeningContributionDto    `json:"contribution,omitempty"`
}

type SymbolScreeningSegmentFindingDto struct {
	StartTime        time.Time `json:"startTime"`
	EndTime          time.Time `json:"endTime"`
	ReturnRate       float64   `json:"returnRate"`
	MaximumDrawdown  float64   `json:"maximumDrawdown"`
	ClosedTradeCount int       `json:"closedTradeCount"`
	LiquidationCount int       `json:"liquidationCount"`
}

type SymbolScreeningContributionDto struct {
	InMarketShare          float64 `json:"inMarketShare"`
	OverlapShare           float64 `json:"overlapShare"`
	BaselineLongestGapDays int     `json:"baselineLongestGapDays"`
	CombinedLongestGapDays int     `json:"combinedLongestGapDays"`
	GapReductionDays       int     `json:"gapReductionDays"`
}
