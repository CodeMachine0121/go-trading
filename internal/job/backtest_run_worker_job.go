package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// BacktestRunWorkerJob fills this replica's worker places from the queue every few seconds, on every replica; once stopped it cuts the runs in flight off and marks them interrupted.
type BacktestRunWorkerJob struct {
	*repeatingRound
	backtestRunApplication *application.BacktestRunApplication
	finished               chan struct{}
}

func NewBacktestRunWorkerJob(
	backtestRunApplication *application.BacktestRunApplication, interval time.Duration,
) *BacktestRunWorkerJob {
	return &BacktestRunWorkerJob{
		repeatingRound: newRepeatingRound(interval, func(executionContext context.Context) {
			started, claimError := backtestRunApplication.RunClaimedBacktests(executionContext)
			if claimError != nil {
				log.Printf("backtest runs could not be claimed this round: %v", claimError)
			}
			if started > 0 {
				log.Printf("backtest run worker started %d run(s)", started)
			}
		}),
		backtestRunApplication: backtestRunApplication,
		finished:               make(chan struct{}),
	}
}

// Start polls until stopped; the runners are cut off only after polling has ended, so none is claimed after the cut.
func (backtestRunWorkerJob *BacktestRunWorkerJob) Start(executionContext context.Context) {
	backtestRunWorkerJob.repeatingRound.Start(executionContext)

	go func() {
		<-backtestRunWorkerJob.repeatingRound.Finished()
		backtestRunWorkerJob.backtestRunApplication.StopRunners()
		close(backtestRunWorkerJob.finished)
	}()
}

func (backtestRunWorkerJob *BacktestRunWorkerJob) Finished() <-chan struct{} {
	return backtestRunWorkerJob.finished
}
