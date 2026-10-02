package vo

import "time"

// PendingMessageAttemptOutcomeKindVo is what one attempt at sending leaves a message as.
type PendingMessageAttemptOutcomeKindVo string

const (
	PendingMessageAttemptSent PendingMessageAttemptOutcomeKindVo = "sent"
	// PendingMessageAttemptRetry is a failure that may fix itself; the message waits and is tried again.
	PendingMessageAttemptRetry PendingMessageAttemptOutcomeKindVo = "retry"
	// PendingMessageAttemptRefused is a failure that will never fix itself; the message is abandoned and its bot halted.
	PendingMessageAttemptRefused PendingMessageAttemptOutcomeKindVo = "refused"
)

// PendingMessageAttemptOutcomeVo carries a retry's wait or a refusal's halt and abandon reasons, never both.
type PendingMessageAttemptOutcomeVo struct {
	Kind          PendingMessageAttemptOutcomeKindVo
	AttemptCount  int
	NextAttemptAt time.Time
	HaltReason    StrategyBotHaltReasonVo
	AbandonReason PendingMessageAbandonReasonVo
}
