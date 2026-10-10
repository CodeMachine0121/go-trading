package domains

import (
	"context"
	"errors"
	"time"
)

// errReplayTimeAllowanceSpent tells the replay's own allowance running out apart from the caller giving up.
var errReplayTimeAllowanceSpent = errors.New("replay time allowance spent")

// ReplayAllowanceDomain is the time one whole replay may take, reading plus every script run; zero leaves only the caller's context to bound it.
type ReplayAllowanceDomain struct {
	duration time.Duration
}

func NewReplayAllowanceDomain(duration time.Duration) ReplayAllowanceDomain {
	return ReplayAllowanceDomain{duration: max(duration, 0)}
}

func (replayAllowanceDomain ReplayAllowanceDomain) Bound(
	executionContext context.Context,
) (context.Context, context.CancelFunc) {
	if replayAllowanceDomain.duration == 0 {
		return context.WithCancel(executionContext)
	}

	return context.WithTimeoutCause(executionContext, replayAllowanceDomain.duration, errReplayTimeAllowanceSpent)
}

// RefusalFor names the allowance first, even when it ran out while the replay waited for a compartment.
func (replayAllowanceDomain ReplayAllowanceDomain) RefusalFor(replayContext context.Context, executionError error) error {
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return BacktestTimeAllowanceSpent(replayAllowanceDomain.duration)
	}

	return executionError
}
