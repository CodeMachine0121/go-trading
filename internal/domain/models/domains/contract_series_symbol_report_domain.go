package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ContractSeriesSymbolReportDomain accumulates one round's outcome for one contract's non-K-candle series; every skip is counted but only the first few hundred are named.
type ContractSeriesSymbolReportDomain struct {
	symbol                  string
	storedCount             int
	skippedCount            int
	skippedRecords          []dto.SkippedRecordDto
	skippedRecordsTruncated bool
	fetchFailureReason      string
}

func NewContractSeriesSymbolReportDomain(symbol string) *ContractSeriesSymbolReportDomain {
	return &ContractSeriesSymbolReportDomain{
		symbol:         symbol,
		skippedRecords: make([]dto.SkippedRecordDto, 0),
	}
}

func (reportDomain *ContractSeriesSymbolReportDomain) NoteStored(storedCount int) {
	reportDomain.storedCount += storedCount
}

func (reportDomain *ContractSeriesSymbolReportDomain) NoteSkipped(recordTime time.Time, reason string) {
	reportDomain.skippedCount++

	if len(reportDomain.skippedRecords) >= maxNamedSkippedKCandles {
		reportDomain.skippedRecordsTruncated = true

		return
	}

	reportDomain.skippedRecords = append(reportDomain.skippedRecords,
		dto.SkippedRecordDto{RecordTime: recordTime.UTC(), Reason: reason})
}

func (reportDomain *ContractSeriesSymbolReportDomain) NoteFetchFailure(reason string) {
	reportDomain.fetchFailureReason = reason
}

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
