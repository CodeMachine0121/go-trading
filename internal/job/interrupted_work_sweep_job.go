package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// InterruptedWorkSweepInterval is long enough that a replica gone this long has missed several heartbeats.
const InterruptedWorkSweepInterval = time.Minute

// InterruptedWorkSweepJob marks as failed what a vanished replica left running, so nobody has to wait for a restart to see it; it works only on duty.
type InterruptedWorkSweepJob struct {
	*repeatingRound
}

func NewInterruptedWorkSweepJob(
	interruptedWorkApplication *application.InterruptedWorkApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *InterruptedWorkSweepJob {
	return &InterruptedWorkSweepJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}

			interrupted, sweepError := interruptedWorkApplication.FailWorkOfVanishedReplicas(executionContext)
			if sweepError != nil {
				log.Printf("work of vanished replicas could not be swept: %v", sweepError)

				return
			}
			if interrupted != (dto.InterruptedWorkDto{}) {
				log.Printf("swept work of vanished replicas: %d answer(s), %d history sync(s), %d contract history sync(s)",
					interrupted.Answers, interrupted.HistorySyncs, interrupted.ContractHistorySyncs)
			}
		})}
}
