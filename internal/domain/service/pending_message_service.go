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

// pendingMessageReadLimit bounds one look at the queue; a person with a long backlog only delays others by one look.
const pendingMessageReadLimit = 500

// pendingMessageExpiredReason is recorded on a message given up because it was too late to matter.
const pendingMessageExpiredReason = "expired"

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
func (pendingMessageService *PendingMessageService) DispatchPendingMessages(
	executionContext context.Context,
) (int, error) {
	now := pendingMessageService.clockProxy.Now()

	// Trimmed here rather than by its own schedule, so there is no second job that could stop running.
	if trimError := pendingMessageService.pendingMessageRepository.DeleteSettledBefore(
		executionContext, now.Add(-domains.PendingMessageRetention)); trimError != nil {
		return 0, trimError
	}

	unsettled, findError := pendingMessageService.pendingMessageRepository.FindUnsettled(
		executionContext, pendingMessageReadLimit)
	if findError != nil {
		return 0, findError
	}

	heads := domains.NewPendingMessageQueueDomain(unsettled).DueAt(now)

	deliveredCount := 0
	deliveredMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	deliverySlots := make(chan struct{}, pendingMessageService.maxConcurrentDeliveries)

	for _, head := range heads {
		waitGroup.Add(1)
		deliverySlots <- struct{}{}

		go func() {
			defer waitGroup.Done()
			defer func() { <-deliverySlots }()

			claimed, claimError := pendingMessageService.pendingMessageRepository.Claim(
				executionContext, head.ID(), pendingMessageService.replicaName, now,
				now.Add(pendingMessageService.sendTimeout))
			if claimError != nil {
				log.Printf("pending message %d could not be taken: %v", head.ID(), claimError)

				return
			}
			if !claimed {
				return
			}
			if head.IsBeingSentAgain() {
				log.Printf("pending message %d is being sent again: the replica sending it never said how it went", head.ID())
			}

			// The signal is forgotten before giving up, so a crash between the two can only cause a resend, never a lost signal.
			if head.IsExpiredAt(now) {
				if head.ForgetsSignalWhenAbandoned() {
					if forgetError := pendingMessageService.strategyBotRepository.ForgetSentSignal(
						executionContext, head.StrategyBotID(), head.Signal()); forgetError != nil {
						log.Printf("pending message %d expired but its bot could not forget the signal: %v",
							head.ID(), forgetError)

						return
					}
				}

				if abandonError := pendingMessageService.pendingMessageRepository.Abandon(
					executionContext, head.ID(), pendingMessageService.replicaName, pendingMessageExpiredReason,
					now); abandonError != nil {
					log.Printf("pending message %d could not be given up: %v", head.ID(), abandonError)

					return
				}

				log.Printf("pending message %d given up: not sent before it stopped mattering", head.ID())

				return
			}

			deliveryResult, deliverError := pendingMessageService.telegramDeliveryService.DeliverPendingMessage(
				executionContext, head.RecipientUserID(), head.Text())
			outcome := head.AfterAttempt(deliveryResult, deliverError, now)

			switch outcome.Kind {
			case vo.PendingMessageAttemptSent:
				if markError := pendingMessageService.pendingMessageRepository.MarkSent(
					executionContext, head.ID(), pendingMessageService.replicaName, now); markError != nil {
					// Left sending, so it is sent again once the claim runs out: a repeat beats a silent loss.
					log.Printf("pending message %d was sent but could not be marked so: %v", head.ID(), markError)
				}

				deliveredMutex.Lock()
				defer deliveredMutex.Unlock()
				deliveredCount++
			case vo.PendingMessageAttemptRefused:
				if haltError := pendingMessageService.haltBot(
					executionContext, head.StrategyBotID(), outcome.HaltReason); haltError != nil {
					log.Printf("pending message %d was refused but its bot could not be halted: %v",
						head.ID(), haltError)

					return
				}

				if abandonError := pendingMessageService.pendingMessageRepository.Abandon(
					executionContext, head.ID(), pendingMessageService.replicaName, string(outcome.HaltReason),
					now); abandonError != nil {
					log.Printf("pending message %d could not be given up: %v", head.ID(), abandonError)

					return
				}

				log.Printf("pending message %d given up and its bot halted: %s", head.ID(), outcome.HaltReason)
			default:
				if rescheduleError := pendingMessageService.pendingMessageRepository.Reschedule(
					executionContext, head.ID(), pendingMessageService.replicaName, outcome.AttemptCount,
					outcome.NextAttemptAt); rescheduleError != nil {
					log.Printf("pending message %d could not be put back to wait: %v", head.ID(), rescheduleError)
				}
			}
		}()
	}

	waitGroup.Wait()

	return deliveredCount, nil
}

// haltBot stops a running bot under the row lock a round being booked also takes, so neither overwrites the other; a bot already stopped is left as it is.
// It exists to scope that transaction: the lock must be let go before the message is settled.
func (pendingMessageService *PendingMessageService) haltBot(
	executionContext context.Context, strategyBotID uint, haltReason vo.StrategyBotHaltReasonVo,
) error {
	return pendingMessageService.transactionRepository.Atomically(executionContext,
		func(transactionContext context.Context) error {
			bot, findError := pendingMessageService.strategyBotRepository.FindOneLocked(
				transactionContext, strategyBotID)
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
				transactionContext, runState.Halt(haltReason))
		})
}
