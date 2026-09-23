package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ContractSeriesSymbolReportDomain accumulates what one round did for one contract
// while fetching a series that is not K candles.
//
// Every refused record is counted, but only the first few hundred are named: a venue
// that starts sending something this system refuses will send it for every record of
// a long backfill, and a report thousands of reasons long is one nobody reads — the
// same limit the K candle report keeps.
type ContractSeriesSymbolReportDomain struct {
	symbol                  string
	storedCount             int
	skippedCount            int
	skippedRecords          []dto.SkippedRecordDto
	skippedRecordsTruncated bool
	fetchFailureReason      string
}

// NewContractSeriesSymbolReportDomain starts the report of one contract.
func NewContractSeriesSymbolReportDomain(symbol string) *ContractSeriesSymbolReportDomain {
	return &ContractSeriesSymbolReportDomain{
		symbol:         symbol,
		skippedRecords: make([]dto.SkippedRecordDto, 0),
	}
}

// NoteStored adds to how many records this round stored.
func (reportDomain *ContractSeriesSymbolReportDomain) NoteStored(storedCount int) {
	reportDomain.storedCount += storedCount
}

// NoteSkipped records one refused record and the rule it broke.
func (reportDomain *ContractSeriesSymbolReportDomain) NoteSkipped(recordTime time.Time, reason string) {
	reportDomain.skippedCount++

	if len(reportDomain.skippedRecords) >= maxNamedSkippedKCandles {
		reportDomain.skippedRecordsTruncated = true

		return
	}

	reportDomain.skippedRecords = append(reportDomain.skippedRecords,
		dto.SkippedRecordDto{RecordTime: recordTime.UTC(), Reason: reason})
}

// NoteFetchFailure records that the venue, or storage, would not answer for this
// contract, and why.
func (reportDomain *ContractSeriesSymbolReportDomain) NoteFetchFailure(reason string) {
	reportDomain.fetchFailureReason = reason
}

// ToDto hands the report outwards.
func (reportDomain *ContractSeriesSymbolReportDomain) ToDto() dto.ContractSeriesSymbolReportDto {
	return dto.ContractSeriesSymbolReportDto{
		Symbol:                  reportDomain.symbol,
		StoredCount:             reportDomain.storedCount,
		SkippedCount:            reportDomain.skippedCount,
		SkippedRecords:          reportDomain.skippedRecords,
		SkippedRecordsTruncated: reportDomain.skippedRecordsTruncated,
		FetchFailureReason:      reportDomain.fetchFailureReason,
	}
}
