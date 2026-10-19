package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// SymbolScreeningRetentionInterval is hourly: runs are kept for months and data for weeks.
const SymbolScreeningRetentionInterval = time.Hour

// SymbolScreeningRetentionJob clears out ended runs and screening market data past their retention, on duty only.
type SymbolScreeningRetentionJob struct {
	*repeatingRound
}

func NewSymbolScreeningRetentionJob(
	screeningApplication *application.SymbolScreeningApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *SymbolScreeningRetentionJob {
	return &SymbolScreeningRetentionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}
			deletedRuns, deletedMarketData, deleteError := screeningApplication.DeleteExpired(executionContext)
			if deleteError != nil {
				log.Printf("expired symbol screening data could not be cleared: %v", deleteError)
				return
			}
			if deletedRuns > 0 || deletedMarketData > 0 {
				log.Printf("cleared %d expired symbol screening run(s) and %d market data row(s)",
					deletedRuns, deletedMarketData)
			}
		})}
}
