package dto

import "time"

// ContractSeriesIngestionReportDto is what one round of fetching a contract series
// that is not K candles — funding rate settlements, position statistics — did, one
// report per contract.
type ContractSeriesIngestionReportDto struct {
	SymbolReports []ContractSeriesSymbolReportDto `json:"symbolReports"`
}

// ContractSeriesSymbolReportDto is what one round did for one contract: how many
// records it stored, which it refused and why, and — when the venue would not answer
// — what it said. A report with no failure reason and nothing stored is an ordinary
// round with nothing new.
type ContractSeriesSymbolReportDto struct {
	Symbol                  string             `json:"symbol"`
	StoredCount             int                `json:"storedCount"`
	SkippedCount            int                `json:"skippedCount"`
	SkippedRecords          []SkippedRecordDto `json:"skippedRecords"`
	SkippedRecordsTruncated bool               `json:"skippedRecordsTruncated"`
	FetchFailureReason      string             `json:"fetchFailureReason"`
}

// SkippedRecordDto is one record a round refused to store: the moment it describes and
// the rule it broke.
type SkippedRecordDto struct {
	RecordTime time.Time `json:"recordTime"`
	Reason     string    `json:"reason"`
}
