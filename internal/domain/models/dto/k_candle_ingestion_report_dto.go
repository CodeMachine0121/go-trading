package dto

import "time"

// KCandleIngestionReportDto is what one ingestion run — a periodic round or the
// startup backfill — has to say for itself, one entry per trading symbol.
type KCandleIngestionReportDto struct {
	SymbolReports []KCandleSymbolIngestionReportDto `json:"symbolReports"`
}

// KCandleSymbolIngestionReportDto is one trading symbol's outcome. An empty
// FetchFailureReason means the source answered; candles it answered with may still
// have been skipped, which is a different thing from not answering at all.
type KCandleSymbolIngestionReportDto struct {
	Symbol string `json:"symbol"`
	// Market is which market this symbol belongs to, so that a reader of the report
	// can tell a market that was shut from one that would not answer.
	Market string `json:"market"`
	// WasAsked says the source was actually reached for this symbol. It is false when
	// the market could hold nothing in the window — a night, a weekend, a day already
	// decided shut — which is a normal state and not a failure worth writing down.
	//
	// It exists because "asked and told nothing" and "never asked" look identical from
	// the counts alone, and only the first of them says anything about the market.
	WasAsked           bool                `json:"wasAsked"`
	StoredCount        int                 `json:"storedCount"`
	SkippedKCandles    []SkippedKCandleDto `json:"skippedKCandles"`
	FetchFailureReason string              `json:"fetchFailureReason"`
}

// SkippedKCandleDto names one K candle that did not make it in, and why. It is
// this precise so that a report says which candle broke which rule rather than
// only that something went wrong.
type SkippedKCandleDto struct {
	OpenTime time.Time `json:"openTime"`
	Reason   string    `json:"reason"`
}
