package service

import (
	"context"
	"errors"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// errReplayTimeAllowanceSpent distinguishes the replay's own allowance running out from the caller giving up.
var errReplayTimeAllowanceSpent = errors.New("replay time allowance spent")

// replayAllowance is the time one whole replay may take, reading plus every script run; zero leaves only the caller's context to bound it.
type replayAllowance struct {
	duration time.Duration
}

func newReplayAllowance(duration time.Duration) replayAllowance {
	return replayAllowance{duration: max(duration, 0)}
}

func (replayAllowance replayAllowance) bound(executionContext context.Context) (context.Context, context.CancelFunc) {
	if replayAllowance.duration == 0 {
		return context.WithCancel(executionContext)
	}

	return context.WithTimeoutCause(executionContext, replayAllowance.duration, errReplayTimeAllowanceSpent)
}

// refusalFor explains an unfinished replay; the allowance running out is named first, even when it ran out waiting for a compartment.
func (replayAllowance replayAllowance) refusalFor(replayContext context.Context, executionError error) error {
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return domains.BacktestTimeAllowanceSpent(replayAllowance.duration)
	}

	return executionError
}
