package dto

import "time"

// KCandleIngestionReportDto covers one ingestion run (periodic round or startup backfill),
// one entry per trading symbol.
type KCandleIngestionReportDto struct {
	SymbolReports []KCandleSymbolIngestionReportDto `json:"symbolReports"`
}

// KCandleSymbolIngestionReportDto with an empty FetchFailureReason means the source
// answered, though some candles may still have been skipped.
type KCandleSymbolIngestionReportDto struct {
	Symbol string `json:"symbol"`
	Market string `json:"market"`
	// WasAsked is false when the market could hold nothing in the window (night, weekend,
	// closed day), distinguishing never-asked from asked-and-got-nothing.
	WasAsked    bool `json:"wasAsked"`
	StoredCount int  `json:"storedCount"`
	// SkippedCount is the true total while SkippedKCandles is capped;
	// SkippedKCandlesTruncated says when they differ.
	SkippedCount             int                 `json:"skippedCount"`
	SkippedKCandles          []SkippedKCandleDto `json:"skippedKCandles"`
	SkippedKCandlesTruncated bool                `json:"skippedKCandlesTruncated"`
	FetchFailureReason       string              `json:"fetchFailureReason"`
}

type SkippedKCandleDto struct {
	OpenTime time.Time `json:"openTime"`
	Reason   string    `json:"reason"`
}
