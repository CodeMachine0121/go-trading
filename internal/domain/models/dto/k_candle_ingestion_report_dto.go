package dto

import "time"

// KCandleIngestionReportDto is what one ingestion run — a periodic round or the
// startup backfill — has to say for itself, one entry per trading symbol.
type KCandleIngestionReportDto struct {
	SymbolReports []KCandleSymbolIngestionReportDto
}

// KCandleSymbolIngestionReportDto is one trading symbol's outcome. An empty
// FetchFailureReason means the source answered; candles it answered with may still
// have been skipped, which is a different thing from not answering at all.
type KCandleSymbolIngestionReportDto struct {
	Symbol             string
	StoredCount        int
	SkippedKCandles    []SkippedKCandleDto
	FetchFailureReason string
}

// SkippedKCandleDto names one K candle that did not make it in, and why. It is
// this precise so that a report says which candle broke which rule rather than
// only that something went wrong.
type SkippedKCandleDto struct {
	OpenTime time.Time
	Reason   string
}
