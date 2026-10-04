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
	"github.com/shopspring/decimal"
)

// contractAutoOrderReadLimit bounds one look at the queue; each bot contributes only its oldest order.
const contractAutoOrderReadLimit = 200

// errContractAutoOrderClaimLost ends a write when another replica has since taken the order, so nothing of this replica's lands.
var errContractAutoOrderClaimLost = errors.New("contract auto order is no longer held by this replica")

// ContractAutoOrderService carries out queued auto orders, from any replica, each step at most once at the venue.
type ContractAutoOrderService struct {
	contractAutoOrderRepository     domaininterface.IContractAutoOrderRepository
	strategyBotRepository           domaininterface.IStrategyBotRepository
	binanceTradingKeyRepository     domaininterface.IBinanceTradingKeyRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	pendingMessageRepository        domaininterface.IPendingMessageRepository
	transactionRepository           domaininterface.ITransactionRepository
	secretSealProxy                 domaininterface.ISecretSealProxy
	contractOrderProxy              domaininterface.IContractOrderProxy
	tradingKeyVerificationProxy     domaininterface.ITradingKeyVerificationProxy
	clockProxy                      domaininterface.IClockProxy
	replicaName                     string
	// executionTimeout is how long a replica holds an order it is carrying out; past it, another replica resumes it.
	executionTimeout        time.Duration
	maxConcurrentExecutions int
}

func NewContractAutoOrderService(
	contractAutoOrderRepository domaininterface.IContractAutoOrderRepository,
	strategyBotRepository domaininterface.IStrategyBotRepository,
	binanceTradingKeyRepository domaininterface.IBinanceTradingKeyRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	pendingMessageRepository domaininterface.IPendingMessageRepository,
	transactionRepository domaininterface.ITransactionRepository,
	secretSealProxy domaininterface.ISecretSealProxy,
	contractOrderProxy domaininterface.IContractOrderProxy,
	tradingKeyVerificationProxy domaininterface.ITradingKeyVerificationProxy,
	clockProxy domaininterface.IClockProxy,
	replicaName string,
	executionTimeout time.Duration,
	maxConcurrentExecutions int,
) *ContractAutoOrderService {
	return &ContractAutoOrderService{
		contractAutoOrderRepository:     contractAutoOrderRepository,
		strategyBotRepository:           strategyBotRepository,
		binanceTradingKeyRepository:     binanceTradingKeyRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		pendingMessageRepository:        pendingMessageRepository,
		transactionRepository:           transactionRepository,
		secretSealProxy:                 secretSealProxy,
		contractOrderProxy:              contractOrderProxy,
		tradingKeyVerificationProxy:     tradingKeyVerificationProxy,
		clockProxy:                      clockProxy,
		replicaName:                     replicaName,
		executionTimeout:                executionTimeout,
		maxConcurrentExecutions:         max(maxConcurrentExecutions, 1),
	}
}

// ExecuteDueAutoOrders carries out each bot's oldest due order and reports how many it settled.
// A failure on one order is logged and left for its claim to run out, so it never stops the others.
func (contractAutoOrderService *ContractAutoOrderService) ExecuteDueAutoOrders(
	executionContext context.Context,
) (int, error) {
	candidates, findError := contractAutoOrderService.contractAutoOrderRepository.FindDispatchCandidates(
		executionContext, contractAutoOrderReadLimit)
	if findError != nil {
		return 0, findError
	}

	lookedAt := contractAutoOrderService.clockProxy.Now()
	settledCount := 0
	settledMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	executionSlots := make(chan struct{}, contractAutoOrderService.maxConcurrentExecutions)

	for _, candidate := range candidates {
		order := domains.NewContractAutoOrderDomain(candidate)
		if !order.IsDispatchableAt(lookedAt) {
			continue
		}

		waitGroup.Add(1)
		executionSlots <- struct{}{}

		go func() {
			defer waitGroup.Done()
			defer func() { <-executionSlots }()

			settled, executeError := contractAutoOrderService.execute(executionContext, order)
			if executeError != nil {
				log.Printf("contract auto order %d stopped for now: %v", order.ID(), executeError)

				return
			}
			if !settled {
				return
			}

			settledMutex.Lock()
			defer settledMutex.Unlock()
			settledCount++
		}()
	}

	waitGroup.Wait()

	return settledCount, nil
}

// execute claims the order and walks it through its steps until it settles or has to wait; it reports whether it settled.
func (contractAutoOrderService *ContractAutoOrderService) execute(
	executionContext context.Context, order domains.ContractAutoOrderDomain,
) (bool, error) {
	claimedAt := contractAutoOrderService.clockProxy.Now()
	claimedUntil := claimedAt.Add(contractAutoOrderService.executionTimeout)

	claimed, claimError := contractAutoOrderService.contractAutoOrderRepository.Claim(
		executionContext, order.ID(), contractAutoOrderService.replicaName, claimedAt, claimedUntil)
	if claimError != nil || !claimed {
		return false, claimError
	}

	// The venue calls stop short of the claim, so this replica never acts on an order another has taken over.
	callContext, endCalls := context.WithDeadline(executionContext, claimedUntil.Add(-time.Second))
	defer endCalls()

	run, prepareError := contractAutoOrderService.prepare(executionContext,
		order.ClaimedBy(contractAutoOrderService.replicaName, claimedUntil))
	if prepareError != nil {
		return false, prepareError
	}

	for {
		outcome := stepSettled
		stepError := error(nil)

		switch run.order.NextStep(run.position) {
		case domains.ContractAutoOrderStepClose:
			outcome, stepError = contractAutoOrderService.close(callContext, &run)
		case domains.ContractAutoOrderStepOpen:
			outcome, stepError = contractAutoOrderService.open(callContext, &run)
		case domains.ContractAutoOrderStepProtect:
			outcome, stepError = contractAutoOrderService.protect(callContext, &run)
		default:
			stepError = contractAutoOrderService.settle(executionContext, run,
				run.order.Settled(run.position, contractAutoOrderService.clockProxy.Now()), nil, false)
		}

		if stepError != nil || outcome != stepContinues {
			return outcome == stepSettled && stepError == nil, stepError
		}
	}
}

// prepare reads everything the steps decide on; a missing bot means it was deleted and its orders with it, so there is nothing left to do.
func (contractAutoOrderService *ContractAutoOrderService) prepare(
	executionContext context.Context, order domains.ContractAutoOrderDomain,
) (orderRun, error) {
	bot, findBotError := contractAutoOrderService.strategyBotRepository.FindOne(executionContext, order.StrategyBotID())
	if findBotError != nil {
		return orderRun{}, findBotError
	}

	run := orderRun{
		order:            order,
		position:         bot.ToAutoOrderPositionVo(),
		bot:              bot,
		tickSize:         decimal.Zero,
		botRunning:       bot.RunState == string(vo.StrategyBotRunning),
		autoOrderEnabled: bot.AutoOrderEnabled,
	}

	contractTradingSymbol, isRegistered, findSymbolError := contractAutoOrderService.contractTradingSymbolRepository.
		FindBySymbol(executionContext, order.Symbol())
	if findSymbolError == nil && isRegistered && contractTradingSymbol.TickSize.Valid {
		run.tickSize = contractTradingSymbol.TickSize.Decimal
	}

	binanceTradingKey, findKeyError := contractAutoOrderService.binanceTradingKeyRepository.FindOneByUser(
		executionContext, order.OwnerUserID())
	if errors.Is(findKeyError, domains.ErrBinanceTradingKeyNotConfigured) {
		return run, nil
	}
	if findKeyError != nil {
		return orderRun{}, findKeyError
	}
	if !binanceTradingKey.ContractTradingEnabled {
		return run, nil
	}

	apiKey, unsealApiKeyError := contractAutoOrderService.secretSealProxy.Unseal(binanceTradingKey.SealedApiKey)
	secretKey, unsealSecretKeyError := contractAutoOrderService.secretSealProxy.Unseal(binanceTradingKey.SealedSecretKey)
	if unsealError := errors.Join(unsealApiKeyError, unsealSecretKeyError); unsealError != nil {
		log.Printf("contract auto order %d: the trading key could not be opened: %v", order.ID(), unsealError)
		run.unsealable = true

		return run, nil
	}

	run.credential = vo.TradingKeyCredentialVo{ApiKey: apiKey, SecretKey: secretKey}
	run.hasCredential = true

	return run, nil
}

// close takes down the bot's protective orders and closes what it opened, unless the venue already has it closed under this order's id.
func (contractAutoOrderService *ContractAutoOrderService) close(
	callContext context.Context, run *orderRun,
) (contractAutoOrderStepOutcome, error) {
	if run.hasCredential {
		fill, findCall := contractAutoOrderService.contractOrderProxy.FindMarketOrder(
			callContext, run.credential, run.order.Symbol(), run.order.CloseClientOrderID())
		if findCall.Failure == vo.ContractOrderFailureNone {
			return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
				run.order, run.position = run.order.AfterClose(run.position, fill)
			})
		}
		if findCall.Failure != vo.ContractOrderFailureNotFound {
			return contractAutoOrderService.fail(callContext, *run, findCall)
		}
	}

	if outcome, refuseError := contractAutoOrderService.refuseToSend(callContext, run); outcome != stepContinues || refuseError != nil {
		return outcome, refuseError
	}

	for _, clientID := range run.order.ProtectiveClientIDsOf(run.position) {
		cancelCall := contractAutoOrderService.contractOrderProxy.CancelProtectiveOrder(
			callContext, run.credential, run.order.Symbol(), clientID)
		if cancelCall.Failure != vo.ContractOrderFailureNone {
			return contractAutoOrderService.fail(callContext, *run, cancelCall)
		}
	}

	exchangePosition, positionCall := contractAutoOrderService.contractOrderProxy.ReadPosition(
		callContext, run.credential, run.order.Symbol())
	if positionCall.Failure != vo.ContractOrderFailureNone {
		return contractAutoOrderService.fail(callContext, *run, positionCall)
	}

	closeOrder, hasSomethingToClose := run.order.CloseOrderFor(run.position, exchangePosition)
	if !hasSomethingToClose {
		return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
			run.order, run.position = run.order.AfterCloseVanished(run.position)
		})
	}

	fill, placeCall := contractAutoOrderService.contractOrderProxy.PlaceMarketOrder(callContext, run.credential, closeOrder)
	if placeCall.Failure != vo.ContractOrderFailureNone {
		return contractAutoOrderService.fail(callContext, *run, placeCall)
	}

	return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
		run.order, run.position = run.order.AfterClose(run.position, fill)
	})
}

// open sets the contract to isolated margin at the bot's leverage and opens the target side, unless the venue already has it opened under this order's id.
func (contractAutoOrderService *ContractAutoOrderService) open(
	callContext context.Context, run *orderRun,
) (contractAutoOrderStepOutcome, error) {
	if run.hasCredential {
		fill, findCall := contractAutoOrderService.contractOrderProxy.FindMarketOrder(
			callContext, run.credential, run.order.Symbol(), run.order.OpenClientOrderID())
		if findCall.Failure == vo.ContractOrderFailureNone {
			return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
				run.order, run.position = run.order.AfterOpen(
					run.position, fill, run.tickSize, contractAutoOrderService.clockProxy.Now())
			})
		}
		if findCall.Failure != vo.ContractOrderFailureNotFound {
			return contractAutoOrderService.fail(callContext, *run, findCall)
		}
	}

	if outcome, refuseError := contractAutoOrderService.refuseToSend(callContext, run); outcome != stepContinues || refuseError != nil {
		return outcome, refuseError
	}

	prepareCall := contractAutoOrderService.contractOrderProxy.PrepareIsolatedLeverage(
		callContext, run.credential, run.order.Symbol(), run.order.OpenLeverage())
	if prepareCall.Failure != vo.ContractOrderFailureNone {
		return contractAutoOrderService.fail(callContext, *run, prepareCall)
	}

	fill, placeCall := contractAutoOrderService.contractOrderProxy.PlaceMarketOrder(
		callContext, run.credential, run.order.OpenOrder())
	if placeCall.Failure != vo.ContractOrderFailureNone {
		return contractAutoOrderService.fail(callContext, *run, placeCall)
	}

	return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
		run.order, run.position = run.order.AfterOpen(
			run.position, fill, run.tickSize, contractAutoOrderService.clockProxy.Now())
	})
}

// protect places whichever protective orders the venue has no record of; one that cannot get through is retried for a while, then left to the owner.
// The brakes are not consulted here: the position is already open, and leaving it unguarded is never what pulling a brake means.
func (contractAutoOrderService *ContractAutoOrderService) protect(
	callContext context.Context, run *orderRun,
) (contractAutoOrderStepOutcome, error) {
	placed := map[vo.ContractProtectiveOrderKindVo]bool{}

	for _, protectiveOrder := range run.order.ProtectiveOrders() {
		if !run.hasCredential {
			continue
		}

		findCall := contractAutoOrderService.contractOrderProxy.FindProtectiveOrder(
			callContext, run.credential, protectiveOrder.Symbol, protectiveOrder.ClientOrderID)
		call := findCall
		if findCall.Failure == vo.ContractOrderFailureNotFound {
			call = contractAutoOrderService.contractOrderProxy.PlaceProtectiveOrder(
				callContext, run.credential, protectiveOrder)
		}

		switch {
		case call.Failure == vo.ContractOrderFailureNone:
			placed[protectiveOrder.Kind] = true
		case call.Failure == vo.ContractOrderFailureUncertain &&
			run.order.MayKeepTryingProtectionAt(contractAutoOrderService.clockProxy.Now()):
			return stepWaits, contractAutoOrderService.reschedule(callContext, *run)
		default:
			log.Printf("contract auto order %d: a protective order was not placed: %s %s",
				run.order.ID(), call.Failure, call.ExchangeMessage)
		}
	}

	return stepContinues, contractAutoOrderService.recordStep(callContext, run, func() {
		run.order, run.position = run.order.AfterProtection(run.position, placed)
	})
}

// refuseToSend is asked right before a market order the venue has no record of; it settles the order when none may go out, and checks once that the account holds one net position.
func (contractAutoOrderService *ContractAutoOrderService) refuseToSend(
	callContext context.Context, run *orderRun,
) (contractAutoOrderStepOutcome, error) {
	now := contractAutoOrderService.clockProxy.Now()
	if run.unsealable {
		return stepSettled, contractAutoOrderService.settle(callContext, *run, run.order.SettledUnsealable(now), nil, false)
	}
	if refused, isRefused := run.order.RefusalToSend(
		now, run.botRunning, run.autoOrderEnabled, run.hasCredential); isRefused {
		return stepSettled, contractAutoOrderService.settle(callContext, *run, refused, nil, false)
	}

	if run.accountChecked {
		return stepContinues, nil
	}

	isHedgeMode, modeCall := contractAutoOrderService.contractOrderProxy.ReadPositionMode(callContext, run.credential)
	if modeCall.Failure != vo.ContractOrderFailureNone {
		return contractAutoOrderService.fail(callContext, *run, modeCall)
	}
	if isHedgeMode {
		return stepSettled, contractAutoOrderService.settle(callContext, *run, run.order.SettledInHedgeMode(now), nil, false)
	}
	run.accountChecked = true

	return stepContinues, nil
}

// fail waits on an answer that may come right and settles on a refusal; a rejected key is asked about once more to tell a dead key from a missing contract permission.
func (contractAutoOrderService *ContractAutoOrderService) fail(
	callContext context.Context, run orderRun, call vo.ContractOrderCallVo,
) (contractAutoOrderStepOutcome, error) {
	if call.Failure == vo.ContractOrderFailureUncertain {
		return stepWaits, contractAutoOrderService.reschedule(callContext, run)
	}

	if call.Failure == vo.ContractOrderFailureKeyRejected {
		verification, verifyError := contractAutoOrderService.tradingKeyVerificationProxy.VerifyTradingKey(
			callContext, run.credential)
		switch {
		case verifyError != nil ||
			verification.FailureReason == vo.TradingKeyVerificationFailureUnreachable ||
			verification.FailureReason == vo.TradingKeyVerificationFailureTimedOut:
			return stepWaits, contractAutoOrderService.reschedule(callContext, run)
		case verification.FailureReason == vo.TradingKeyVerificationFailureNone && !verification.ContractTradingEnabled:
			call.Failure = vo.ContractOrderFailureNoContractPermission
		}
	}

	settledOrder, switchOffKinds, switchesOff := run.order.SettledByFailure(call, contractAutoOrderService.clockProxy.Now())

	return stepSettled, contractAutoOrderService.settle(callContext, run, settledOrder, switchOffKinds, switchesOff)
}

// recordStep applies a finished step and writes it back with the bot's position in one transaction, so the two never disagree.
func (contractAutoOrderService *ContractAutoOrderService) recordStep(
	callContext context.Context, run *orderRun, apply func(),
) error {
	apply()

	// Its own deadline, not the calls': a step the venue has carried out must be written even when the calls ran out of time.
	writeContext, endWrite := context.WithTimeout(context.WithoutCancel(callContext), contractAutoOrderService.executionTimeout)
	defer endWrite()

	return contractAutoOrderService.transactionRepository.Atomically(writeContext,
		func(transactionContext context.Context) error {
			held, saveError := contractAutoOrderService.contractAutoOrderRepository.SaveProgress(
				transactionContext, run.order.ToEntity(), contractAutoOrderService.replicaName)
			if saveError != nil {
				return saveError
			}
			if !held {
				return errContractAutoOrderClaimLost
			}

			return contractAutoOrderService.strategyBotRepository.UpdateAutoOrderPosition(
				transactionContext, run.order.StrategyBotID(), run.position)
		})
}

func (contractAutoOrderService *ContractAutoOrderService) reschedule(
	callContext context.Context, run orderRun,
) error {
	writeContext, endWrite := context.WithTimeout(context.WithoutCancel(callContext), contractAutoOrderService.executionTimeout)
	defer endWrite()

	held, saveError := contractAutoOrderService.contractAutoOrderRepository.SaveProgress(
		writeContext, run.order.Rescheduled(contractAutoOrderService.clockProxy.Now()).ToEntity(),
		contractAutoOrderService.replicaName)
	if saveError != nil {
		return saveError
	}
	if !held {
		return errContractAutoOrderClaimLost
	}

	return nil
}

// settle ends the order, tells its owner what happened and, on a key the venue no longer accepts, switches off what that key can no longer back, all in one transaction.
func (contractAutoOrderService *ContractAutoOrderService) settle(
	callContext context.Context, run orderRun, settledOrder domains.ContractAutoOrderDomain,
	switchOffKinds []string, switchesOff bool,
) error {
	writeContext, endWrite := context.WithTimeout(context.WithoutCancel(callContext), contractAutoOrderService.executionTimeout)
	defer endWrite()

	now := contractAutoOrderService.clockProxy.Now()

	return contractAutoOrderService.transactionRepository.Atomically(writeContext,
		func(transactionContext context.Context) error {
			held, saveError := contractAutoOrderService.contractAutoOrderRepository.SaveProgress(
				transactionContext, settledOrder.ToEntity(), contractAutoOrderService.replicaName)
			if saveError != nil {
				return saveError
			}
			if !held {
				return errContractAutoOrderClaimLost
			}

			if switchesOff {
				if switchOffError := contractAutoOrderService.strategyBotRepository.DisableAutoOrderByOwner(
					transactionContext, settledOrder.OwnerUserID(), switchOffKinds); switchOffError != nil {
					return switchOffError
				}
			}

			return contractAutoOrderService.pendingMessageRepository.Enqueue(transactionContext,
				domains.NewAutoOrderPendingMessageDomain(
					settledOrder.StrategyBotID(), settledOrder.OwnerUserID(),
					domains.NewContractAutoOrderMessageDomain(run.bot.Name, settledOrder.Symbol(), settledOrder).Text(),
					now,
				).ToEntity())
		})
}
