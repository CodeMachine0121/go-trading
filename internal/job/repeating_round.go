package job

import (
	"context"
	"sync"
	"time"
)

// repeatingRound runs one round on start and then every interval; the first round runs before the ticker exists so a long catch-up is never overlapped by a scheduled round.
type repeatingRound struct {
	interval time.Duration
	runRound func(executionContext context.Context)
	done     chan struct{}
	// finished closes when the job's goroutine has returned, in-flight round included.
	finished chan struct{}
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
		finished: make(chan struct{}),
		stopOnce: sync.OnceFunc(func() { close(done) }),
	}
}

func (repeatingRound *repeatingRound) Start(executionContext context.Context) {
	go repeatingRound.run(executionContext)
}

// Stop lets a running round finish and is idempotent.
func (repeatingRound *repeatingRound) Stop() {
	repeatingRound.stopOnce()
}

func (repeatingRound *repeatingRound) run(executionContext context.Context) {
	defer close(repeatingRound.finished)

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
			// Re-check stop because select picks randomly when a tick is already pending.
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

// Finished closes once the job has stopped and its in-flight round has ended.
func (repeatingRound *repeatingRound) Finished() <-chan struct{} {
	return repeatingRound.finished
}
