package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_pending_message_repository.go -destination=mocks/mock_i_pending_message_repository.go -package=mocks

// IPendingMessageRepository holds messages waiting to be sent; every write after Enqueue is guarded by who holds the message, so two replicas can never both settle one.
type IPendingMessageRepository interface {
	// Enqueue takes part in a surrounding transaction; a second message for the same round is silently dropped.
	Enqueue(executionContext context.Context, pendingMessage entities.PendingMessage) error

	// FindUnsettled returns ready and sending messages in the order they were queued, at most limit.
	FindUnsettled(executionContext context.Context, limit int) ([]entities.PendingMessage, error)

	// Claim takes the message for claimant when it is ready and due at moment, or its sender's claim ran out by moment; false means another replica has it.
	Claim(
		executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
	) (bool, error)

	MarkSent(executionContext context.Context, id uint, claimant string, settledAt time.Time) error

	// Reschedule hands the message back to wait until nextAttemptAt, having failed attemptCount times.
	Reschedule(
		executionContext context.Context, id uint, claimant string, attemptCount int, nextAttemptAt time.Time,
	) error

	Abandon(executionContext context.Context, id uint, claimant string, reason string, settledAt time.Time) error

	// DeleteSettledBefore drops sent and abandoned messages settled before cutoff.
	DeleteSettledBefore(executionContext context.Context, cutoff time.Time) error
}
