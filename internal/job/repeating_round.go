package job

import (
	"context"
	"sync"
	"time"
)

// repeatingRound is the running half of a job that does one thing on start and then
// again every interval: it holds the ticker's lifetime, the stop signal, and the round
// it runs. It carries no business rule of its own — what a round does, and what it
// reports, is the job's.
//
// **The round on start is not the first tick.** It runs before the ticker exists, so a
// round that takes longer than an interval — a catch-up after a long stop — is never
// joined by a scheduled one halfway through. That is the same order the contract K
// candle job keeps between its backfill and its rounds.
type repeatingRound struct {
	interval time.Duration
	runRound func(executionContext context.Context)
	done     chan struct{}
	stopOnce func()
}

func newRepeatingRound(
	interval time.Duration, runRound func(executionContext context.Context),
) *repeatingRound {
	done := make(chan struct{})

	return &repeatingRound{
		interval: interval,
		runRound: runRound,
		done:     done,
		stopOnce: sync.OnceFunc(func() { close(done) }),
	}
}

// Start runs the first round and then keeps running one every interval, until the
// context ends or Stop is called.
func (repeatingRound *repeatingRound) Start(executionContext context.Context) {
	go repeatingRound.run(executionContext)
}

// Stop ends the rounds. A round already running finishes; none starts after it.
// Calling it more than once is harmless.
func (repeatingRound *repeatingRound) Stop() {
	repeatingRound.stopOnce()
}

// run is Start's goroutine. It stays a method of its own because of what it encloses:
// the ticker, from the moment it is made to the moment it is let go.
func (repeatingRound *repeatingRound) run(executionContext context.Context) {
	repeatingRound.runRound(executionContext)

	ticker := time.NewTicker(repeatingRound.interval)
	defer ticker.Stop()

	for {
		select {
		case <-repeatingRound.done:
			return
		case <-executionContext.Done():
			return
		case <-ticker.C:
			// A tick and a stop can be ready together, and select picks between
			// them at random. Checking again means a job that was stopped never
			// starts one more round on its way out.
			select {
			case <-repeatingRound.done:
				return
			case <-executionContext.Done():
				return
			default:
			}

			repeatingRound.runRound(executionContext)
		}
	}
}
