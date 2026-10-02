package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	// pendingMessageFirstRetryWait doubles with every failed attempt up to pendingMessageLongestRetryWait.
	pendingMessageFirstRetryWait   = 2 * time.Second
	pendingMessageLongestRetryWait = 5 * time.Minute
	// pendingMessageShortestRoundDeadline keeps a one-minute bot's message from being given up within a minute.
	pendingMessageShortestRoundDeadline = 5 * time.Minute
	// pendingMessageLifecycleDeadline is longer because "started" or "stopped" stays true for hours.
	pendingMessageLifecycleDeadline = time.Hour
	// PendingMessageRetention is how long sent and abandoned messages are kept before they are dropped.
	PendingMessageRetention = 7 * 24 * time.Hour
)

// PendingMessageDomain decides what happens to one queued message: when it may be sent, when it is too late, and what one attempt leaves it as.
type PendingMessageDomain struct {
	message entities.PendingMessage
}

// NewPendingMessageDomain reads an unrecognised status as ready, so a damaged row is retried rather than stranded.
func NewPendingMessageDomain(message entities.PendingMessage) PendingMessageDomain {
	switch vo.PendingMessageStatusVo(message.Status) {
	case vo.PendingMessageReady, vo.PendingMessageSending, vo.PendingMessageSent, vo.PendingMessageAbandoned:
	default:
		message.Status = string(vo.PendingMessageReady)
	}

	return PendingMessageDomain{message: message}
}

// NewRoundPendingMessageDomain queues a round's message, giving up after the bot's own interval but never within five minutes, since a signal speaks about now.
func NewRoundPendingMessageDomain(
	strategyBotID uint, recipientUserID uint, roundDueAt time.Time, signal vo.SignalVo, text string,
	triggerInterval time.Duration, now time.Time,
) PendingMessageDomain {
	deadline := max(triggerInterval, pendingMessageShortestRoundDeadline)
	roundDueAtUtc := roundDueAt.UTC()

	return NewPendingMessageDomain(entities.PendingMessage{
		StrategyBotID: strategyBotID, RecipientUserID: recipientUserID, RoundDueAt: &roundDueAtUtc,
		Kind: string(vo.PendingMessageRound), Signal: string(signal), Text: text,
		Status: string(vo.PendingMessageReady), NextAttemptAt: now.UTC(), ExpiresAt: now.UTC().Add(deadline),
		CreatedAt: now.UTC(),
	})
}

// NewLifecyclePendingMessageDomain queues a bot's message about itself.
func NewLifecyclePendingMessageDomain(
	strategyBotID uint, recipientUserID uint, text string, now time.Time,
) PendingMessageDomain {
	return NewPendingMessageDomain(entities.PendingMessage{
		StrategyBotID: strategyBotID, RecipientUserID: recipientUserID,
		Kind: string(vo.PendingMessageLifecycle), Text: text,
		Status: string(vo.PendingMessageReady), NextAttemptAt: now.UTC(),
		ExpiresAt: now.UTC().Add(pendingMessageLifecycleDeadline), CreatedAt: now.UTC(),
	})
}

func (pendingMessageDomain PendingMessageDomain) ToEntity() entities.PendingMessage {
	return pendingMessageDomain.message
}

func (pendingMessageDomain PendingMessageDomain) ID() uint {
	return pendingMessageDomain.message.ID
}

func (pendingMessageDomain PendingMessageDomain) StrategyBotID() uint {
	return pendingMessageDomain.message.StrategyBotID
}

func (pendingMessageDomain PendingMessageDomain) RecipientUserID() uint {
	return pendingMessageDomain.message.RecipientUserID
}

func (pendingMessageDomain PendingMessageDomain) Text() string {
	return pendingMessageDomain.message.Text
}

func (pendingMessageDomain PendingMessageDomain) Signal() string {
	return pendingMessageDomain.message.Signal
}

// IsExpiredAt counts the deadline itself as too late.
func (pendingMessageDomain PendingMessageDomain) IsExpiredAt(now time.Time) bool {
	return !now.Before(pendingMessageDomain.message.ExpiresAt)
}

// IsDispatchableAt is a ready message whose wait is over, or one whose sender's claim has run out, which is how a send cut short by a dying replica is retried.
func (pendingMessageDomain PendingMessageDomain) IsDispatchableAt(now time.Time) bool {
	switch vo.PendingMessageStatusVo(pendingMessageDomain.message.Status) {
	case vo.PendingMessageReady:
		return !pendingMessageDomain.message.NextAttemptAt.After(now)
	case vo.PendingMessageSending:
		claimedUntil := pendingMessageDomain.message.ClaimedUntil

		return claimedUntil == nil || !claimedUntil.After(now)
	default:
		return false
	}
}

// ForgetsSignalWhenAbandoned is true only for a round's message: its bot must not go on believing the owner heard a signal that never arrived.
func (pendingMessageDomain PendingMessageDomain) ForgetsSignalWhenAbandoned() bool {
	return vo.PendingMessageKindVo(pendingMessageDomain.message.Kind) == vo.PendingMessageRound &&
		pendingMessageDomain.message.Signal != ""
}

// AfterAttempt halts only on failures the owner must fix; everything else waits twice as long as last time, or as long as the destination asked if that is longer.
func (pendingMessageDomain PendingMessageDomain) AfterAttempt(
	deliveryResult vo.DeliveryResultVo, now time.Time,
) vo.PendingMessageAttemptOutcomeVo {
	if deliveryResult.FailureReason == vo.DeliveryFailureNone {
		return vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptSent}
	}

	deliveryFailure := NewStrategyBotDeliveryFailureDomain(deliveryResult.FailureReason)
	if deliveryFailure.HaltsTheBot() {
		return vo.PendingMessageAttemptOutcomeVo{
			Kind: vo.PendingMessageAttemptRefused, HaltReason: deliveryFailure.HaltReason(),
		}
	}

	retryWait := pendingMessageFirstRetryWait
	for range pendingMessageDomain.message.AttemptCount {
		retryWait *= 2
		if retryWait >= pendingMessageLongestRetryWait {
			retryWait = pendingMessageLongestRetryWait

			break
		}
	}

	return vo.PendingMessageAttemptOutcomeVo{
		Kind:          vo.PendingMessageAttemptRetry,
		AttemptCount:  pendingMessageDomain.message.AttemptCount + 1,
		NextAttemptAt: now.Add(max(retryWait, deliveryResult.RetryAfter)),
	}
}

// AfterMissingDeliverySetting is a refusal: the owner removed where to send, and waiting will not bring it back.
func (pendingMessageDomain PendingMessageDomain) AfterMissingDeliverySetting() vo.PendingMessageAttemptOutcomeVo {
	return vo.PendingMessageAttemptOutcomeVo{
		Kind: vo.PendingMessageAttemptRefused, HaltReason: vo.StrategyBotHaltDeliveryNotConfigured,
	}
}
