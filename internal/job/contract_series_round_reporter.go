package job

import (
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// contractSeriesRoundReporter writes down what a round of one contract series did —
// only what went wrong, because a round that stored what it found is the ordinary
// case and a log line for every one of those would bury the ones that matter.
type contractSeriesRoundReporter struct {
	seriesName string
}

// report writes down a round's failures: the whole round not running, a contract the
// venue would not answer for, and every record that was refused.
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
