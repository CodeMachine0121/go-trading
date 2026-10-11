package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// BacktestRunRetentionInterval is hourly: runs are kept for days, so clearing them out is not urgent.
const BacktestRunRetentionInterval = time.Hour

// BacktestRunRetentionJob clears out runs that ended longer ago than the retention; it works only on duty.
type BacktestRunRetentionJob struct {
	*repeatingRound
}

func NewBacktestRunRetentionJob(
	backtestRunApplication *application.BacktestRunApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *BacktestRunRetentionJob {
	return &BacktestRunRetentionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}

			deleted, deleteError := backtestRunApplication.DeleteExpiredRuns(executionContext)
			if deleteError != nil {
				log.Printf("expired backtest runs could not be cleared: %v", deleteError)

				return
			}
			if deleted > 0 {
				log.Printf("cleared %d expired backtest run(s)", deleted)
			}
		})}
}
