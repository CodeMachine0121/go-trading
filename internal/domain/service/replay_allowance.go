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

// refusalFor explains an unfinished replay, with an actionable message when the allowance ran out.
func (replayAllowance replayAllowance) refusalFor(replayContext context.Context, executionError error) error {
	// Waiting for a compartment is not cured by asking for less, so it is not blamed on the allowance.
	if errors.Is(executionError, domains.ErrIndicatorScriptCompartmentsBusy) {
		return executionError
	}
	if errors.Is(context.Cause(replayContext), errReplayTimeAllowanceSpent) {
		return domains.BacktestTimeAllowanceSpent(replayAllowance.duration)
	}

	return executionError
}
