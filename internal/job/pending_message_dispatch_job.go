package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// PendingMessageDispatchJob sends queued bot messages every few seconds on every replica; a batch cut off mid-send leaves its messages claimed, and they are sent again once the claim runs out.
type PendingMessageDispatchJob struct {
	*repeatingRound
}

func NewPendingMessageDispatchJob(
	pendingMessageDispatchApplication *application.PendingMessageDispatchApplication, interval time.Duration,
) *PendingMessageDispatchJob {
	return &PendingMessageDispatchJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if _, dispatchError := pendingMessageDispatchApplication.DispatchPendingMessages(
				executionContext); dispatchError != nil {
				log.Printf("pending messages could not be sent this round: %v", dispatchError)
			}
		})}
}
