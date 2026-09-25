package job

import (
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// contractSeriesRoundReporter logs only a round's failures.
type contractSeriesRoundReporter struct {
	seriesName string
}

func (roundReporter contractSeriesRoundReporter) report(
	roundReport dto.ContractSeriesIngestionReportDto, roundError error,
) {
	if roundError != nil {
		log.Printf("contract %s round did not run: %v", roundReporter.seriesName, roundError)

		return
	}

	for _, symbolReport := range roundReport.SymbolReports {
		if symbolReport.FetchFailureReason != "" {
			log.Printf("contract %s round got no answer for %s: %s",
				roundReporter.seriesName, symbolReport.Symbol, symbolReport.FetchFailureReason)
		}

		for _, skippedRecord := range symbolReport.SkippedRecords {
			log.Printf("contract %s round skipped %s at %s: %s",
				roundReporter.seriesName, symbolReport.Symbol,
				skippedRecord.RecordTime.Format(time.RFC3339Nano), skippedRecord.Reason)
		}
	}
}
