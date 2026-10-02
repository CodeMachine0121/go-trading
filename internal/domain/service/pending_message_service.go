package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// pendingMessageReadLimit bounds one look at the queue; since each person contributes only their oldest message and anything already expired, it is reached only with this many people waiting at once.
const pendingMessageReadLimit = 500

// pendingMessageTrimInterval spaces out dropping old settled messages, which every replica does and which would otherwise contend with deleting a bot on every tick.
const pendingMessageTrimInterval = time.Minute

// PendingMessageService sends what bots have queued, from any replica, at least once each.
type PendingMessageService struct {
	pendingMessageRepository domaininterface.IPendingMessageRepository
	strategyBotRepository    domaininterface.IStrategyBotRepository
	transactionRepository    domaininterface.ITransactionRepository
	telegramDeliveryService  *TelegramDeliveryService
	clockProxy               domaininterface.IClockProxy
	replicaName              string
	// sendTimeout is how long a replica holds a message it is sending; past it, another replica sends it again.
	sendTimeout             time.Duration
	maxConcurrentDeliveries int

	trimMutex     sync.Mutex
	lastTrimmedAt time.Time
}

func NewPendingMessageService(
	pendingMessageRepository domaininterface.IPendingMessageRepository,
	strategyBotRepository domaininterface.IStrategyBotRepository,
	transactionRepository domaininterface.ITransactionRepository,
	telegramDeliveryService *TelegramDeliveryService,
	clockProxy domaininterface.IClockProxy,
	replicaName string,
	sendTimeout time.Duration,
	maxConcurrentDeliveries int,
) *PendingMessageService {
	return &PendingMessageService{
		pendingMessageRepository: pendingMessageRepository,
		strategyBotRepository:    strategyBotRepository,
		transactionRepository:    transactionRepository,
		telegramDeliveryService:  telegramDeliveryService,
		clockProxy:               clockProxy,
		replicaName:              replicaName,
		sendTimeout:              sendTimeout,
		maxConcurrentDeliveries:  max(maxConcurrentDeliveries, 1),
	}
}

// EnqueueLifecycleMessage queues what a bot says about itself, such as having started or stopped.
func (pendingMessageService *PendingMessageService) EnqueueLifecycleMessage(
	executionContext context.Context, strategyBotID uint, recipientUserID uint, text string,
) error {
	return pendingMessageService.pendingMessageRepository.Enqueue(executionContext,
		domains.NewLifecyclePendingMessageDomain(
			strategyBotID, recipientUserID, text, pendingMessageService.clockProxy.Now()).ToEntity())
}

// DispatchPendingMessages sends each person's oldest due message and reports how many arrived.
// A failure on one message is logged and left for its claim to run out, so it never stops the others.
// Each message reads the clock when its turn comes, so a long batch never claims with a deadline already past.
func (pendingMessageService *PendingMessageService) DispatchPendingMessages(
	executionContext context.Context,
) (int, error) {
	lookedAt := pendingMessageService.clockProxy.Now()

	// Trimmed here rather than by its own schedule, so there is no second job that could stop running.
	if pendingMessageService.isTrimDue(lookedAt) {
		if trimError := pendingMessageService.pendingMessageRepository.DeleteSettledBefore(
			executionContext, lookedAt.Add(-domains.PendingMessageRetention)); trimError != nil {
			return 0, trimError
		}
	}

	candidates, findError := pendingMessageService.pendingMessageRepository.FindDispatchCandidates(
		executionContext, lookedAt, pendingMessageReadLimit)
	if findError != nil {
		return 0, findError
	}

	dueMessages := domains.NewPendingMessageQueueDomain(candidates).DueAt(lookedAt)

	deliveredCount := 0
	deliveredMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	deliverySlots := make(chan struct{}, pendingMessageService.maxConcurrentDeliveries)

	for _, dueMessage := range dueMessages {
		waitGroup.Add(1)
		deliverySlots <- struct{}{}

		go func() {
			defer waitGroup.Done()
			defer func() { <-deliverySlots }()

			claimedAt := pendingMessageService.clockProxy.Now()
			replicaName := pendingMessageService.replicaName

			claimed, claimError := pendingMessageService.pendingMessageRepository.Claim(
				executionContext, dueMessage.ID(), replicaName, claimedAt,
				claimedAt.Add(pendingMessageService.sendTimeout))
			if claimError != nil {
				log.Printf("pending message %d could not be taken: %v", dueMessage.ID(), claimError)

				return
			}
			if !claimed {
				return
			}
			if dueMessage.IsBeingSentAgain() {
				log.Printf("pending message %d is being sent again: the replica sending it never said how it went",
					dueMessage.ID())
			}

			// The signal is forgotten before giving up, so a crash between the two can only cause a resend, never a lost signal.
			if dueMessage.IsExpiredAt(claimedAt) {
				if roundDueAt, speaksForARound := dueMessage.RoundDueAt(); speaksForARound &&
					dueMessage.ForgetsSignalWhenAbandoned() {
					if forgetError := pendingMessageService.strategyBotRepository.ForgetSentSignal(
						executionContext, dueMessage.StrategyBotID(), roundDueAt); forgetError != nil {
						log.Printf("pending message %d expired but its bot could not forget the signal: %v",
							dueMessage.ID(), forgetError)

						return
					}
				}

				if abandonError := pendingMessageService.pendingMessageRepository.Abandon(
					executionContext, dueMessage.ID(), replicaName, vo.PendingMessageAbandonedExpired,
					claimedAt); abandonError != nil {
					log.Printf("pending message %d could not be given up: %v", dueMessage.ID(), abandonError)

					return
				}

				log.Printf("pending message %d given up: not sent before it stopped mattering", dueMessage.ID())

				return
			}

			deliveryResult, deliverError := pendingMessageService.telegramDeliveryService.DeliverPendingMessage(
				executionContext, dueMessage.RecipientUserID(), dueMessage.Text())
			settledAt := pendingMessageService.clockProxy.Now()
			outcome := dueMessage.AfterAttempt(deliveryResult, deliverError, settledAt)

			switch outcome.Kind {
			case vo.PendingMessageAttemptSent:
				if markError := pendingMessageService.pendingMessageRepository.MarkSent(
					executionContext, dueMessage.ID(), replicaName, settledAt); markError != nil {
					// Left sending, so it is sent again once the claim runs out: a repeat beats a silent loss.
					log.Printf("pending message %d was sent but could not be marked so: %v", dueMessage.ID(), markError)
				}

				deliveredMutex.Lock()
				defer deliveredMutex.Unlock()
				deliveredCount++
			case vo.PendingMessageAttemptRefused:
				// Halted under the row lock a round being booked also takes, so neither overwrites the other; a bot already stopped or gone is left as it is.
				haltError := pendingMessageService.transactionRepository.Atomically(executionContext,
					func(transactionContext context.Context) error {
						bot, findError := pendingMessageService.strategyBotRepository.FindOneLocked(
							transactionContext, dueMessage.StrategyBotID())
						if errors.Is(findError, domains.ErrStrategyBotNotFound) {
							return nil
						}
						if findError != nil {
							return findError
						}

						runState := domains.NewStrategyBotRunStateDomain(bot)
						if !runState.IsRunning() {
							return nil
						}

						return pendingMessageService.strategyBotRepository.UpdateRunState(
							transactionContext, runState.Halt(outcome.HaltReason, settledAt))
					})
				if haltError != nil {
					log.Printf("pending message %d was refused but its bot could not be halted: %v",
						dueMessage.ID(), haltError)

					return
				}

				if abandonError := pendingMessageService.pendingMessageRepository.Abandon(
					executionContext, dueMessage.ID(), replicaName, outcome.AbandonReason,
					settledAt); abandonError != nil {
					log.Printf("pending message %d could not be given up: %v", dueMessage.ID(), abandonError)

					return
				}

				log.Printf("pending message %d given up and its bot halted: %s", dueMessage.ID(), outcome.HaltReason)
			default:
				if rescheduleError := pendingMessageService.pendingMessageRepository.Reschedule(
					executionContext, dueMessage.ID(), replicaName, outcome.AttemptCount,
					outcome.NextAttemptAt); rescheduleError != nil {
					log.Printf("pending message %d could not be put back to wait: %v", dueMessage.ID(), rescheduleError)
				}
			}
		}()
	}

	waitGroup.Wait()

	return deliveredCount, nil
}

// isTrimDue exists to scope the trim lock with defer: it decides and records the trim in one hold, so two concurrent looks never both trim.
func (pendingMessageService *PendingMessageService) isTrimDue(now time.Time) bool {
	pendingMessageService.trimMutex.Lock()
	defer pendingMessageService.trimMutex.Unlock()

	if !pendingMessageService.lastTrimmedAt.IsZero() &&
		now.Sub(pendingMessageService.lastTrimmedAt) < pendingMessageTrimInterval {
		return false
	}
	pendingMessageService.lastTrimmedAt = now

	return true
}
