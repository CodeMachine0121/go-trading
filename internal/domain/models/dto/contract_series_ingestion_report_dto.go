package dto

import "time"

// ContractSeriesIngestionReportDto reports one fetch round of non-candle contract series
// (funding settlements, position statistics).
type ContractSeriesIngestionReportDto struct {
	SymbolReports []ContractSeriesSymbolReportDto `json:"symbolReports"`
}

// ContractSeriesSymbolReportDto with no failure reason and nothing stored is a normal round
// with nothing new.
type ContractSeriesSymbolReportDto struct {
	Symbol                  string             `json:"symbol"`
	StoredCount             int                `json:"storedCount"`
	SkippedCount            int                `json:"skippedCount"`
	SkippedRecords          []SkippedRecordDto `json:"skippedRecords"`
	SkippedRecordsTruncated bool               `json:"skippedRecordsTruncated"`
	FetchFailureReason      string             `json:"fetchFailureReason"`
}

type SkippedRecordDto struct {
	RecordTime time.Time `json:"recordTime"`
	Reason     string    `json:"reason"`
}
