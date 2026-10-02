package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ReplicaHeartbeatJob says this replica is alive, and runs even with background jobs switched off:
// such a replica still answers requests that start assistant answers and history syncs, and without a heartbeat other replicas would take that work for interrupted.
type ReplicaHeartbeatJob struct {
	*repeatingRound
}

func NewReplicaHeartbeatJob(
	interruptedWorkApplication *application.InterruptedWorkApplication, interval time.Duration,
) *ReplicaHeartbeatJob {
	return &ReplicaHeartbeatJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if beatError := interruptedWorkApplication.BeatHeartbeat(executionContext); beatError != nil {
				log.Printf("replica heartbeat could not be recorded: %v", beatError)
			}
		})}
}
