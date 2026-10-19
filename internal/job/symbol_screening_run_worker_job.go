package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// SymbolScreeningRunWorkerJob screens on the duty replica only, which keeps the whole system to one run at a time; losing duty or stopping cuts the run off as interrupted.
type SymbolScreeningRunWorkerJob struct {
	*repeatingRound
	screeningApplication *application.SymbolScreeningApplication
	finished             chan struct{}
}

func NewSymbolScreeningRunWorkerJob(
	screeningApplication *application.SymbolScreeningApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *SymbolScreeningRunWorkerJob {
	return &SymbolScreeningRunWorkerJob{
		repeatingRound: newRepeatingRound(interval, func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				screeningApplication.StopRunner()
				return
			}
			started, claimError := screeningApplication.RunClaimedScreening(executionContext)
			if claimError != nil {
				log.Printf("symbol screening runs could not be claimed this round: %v", claimError)
			}
			if started {
				log.Printf("symbol screening worker started a run")
			}
		}),
		screeningApplication: screeningApplication,
		finished:             make(chan struct{}),
	}
}

func (workerJob *SymbolScreeningRunWorkerJob) Start(executionContext context.Context) {
	workerJob.repeatingRound.Start(executionContext)
	go func() {
		<-workerJob.repeatingRound.Finished()
		workerJob.screeningApplication.StopRunner()
		close(workerJob.finished)
	}()
}

func (workerJob *SymbolScreeningRunWorkerJob) Finished() <-chan struct{} {
	return workerJob.finished
}
