package application

import (
	"context"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// strategyBotObservationWindowLength is how much of the clock one round asks about.
//
// A bot never asks about a stretch; it asks what the market is saying now. One
// minute is the shortest window that holds exactly one slot at every coarseness, so
// whatever the source's interval, the answer is the newest finished candle of it and
// nothing more.
const strategyBotObservationWindowLength = time.Minute

// StrategyBotRunApplication runs the rounds that are due, and is the only thing that
// does.
//
// It offers one method. Whoever calls it — today a scan on a timer — sequences
// nothing and knows none of the names below: resolving strategies, running scripts
// side by side, reading conditions, sending a message and deciding what a failure
// means all happen behind it. That is deliberate, because everything on that list is
// a step somebody would otherwise be able to leave out.
type StrategyBotRunApplication struct {
	strategyBotService          *service.StrategyBotService
	strategyService             *service.StrategyService
	indicatorCalculationService *service.IndicatorCalculationService
	telegramDeliveryService     *service.TelegramDeliveryService
	kCandleService              *service.KCandleService
	clockProxy                  domaininterface.IClockProxy
	roundGuard                  *StrategyBotRoundGuard
	maxConcurrentRounds         int
	roundTimeout                time.Duration
}

func NewStrategyBotRunApplication(
	strategyBotService *service.StrategyBotService,
	strategyService *service.StrategyService,
	indicatorCalculationService *service.IndicatorCalculationService,
	telegramDeliveryService *service.TelegramDeliveryService,
	kCandleService *service.KCandleService,
	clockProxy domaininterface.IClockProxy,
	roundGuard *StrategyBotRoundGuard,
	maxConcurrentRounds int,
	roundTimeout time.Duration,
) *StrategyBotRunApplication {
	return &StrategyBotRunApplication{
		strategyBotService:          strategyBotService,
		strategyService:             strategyService,
		indicatorCalculationService: indicatorCalculationService,
		telegramDeliveryService:     telegramDeliveryService,
		kCandleService:              kCandleService,
		clockProxy:                  clockProxy,
		roundGuard:                  roundGuard,
		maxConcurrentRounds:         maxConcurrentRounds,
		roundTimeout:                roundTimeout,
	}
}

// RunDueRounds runs one round for each bot that is due, several at a time, and
// reports how many it ran.
//
// The number due is capped by the read rather than by this loop, so a system coming
// back after a long stop does not pull every bot it owns into memory in order to run
// a handful of them. The ones it leaves are due again a minute later.
//
// One bot failing never stops the others: each round keeps its own failure, and a
// bot halted for a deleted strategy has nothing to do with the bot beside it.
func (strategyBotRunApplication *StrategyBotRunApplication) RunDueRounds(
	executionContext context.Context,
) (int, error) {
	dueBots, findError := strategyBotRunApplication.strategyBotService.FindDueStrategyBots(
		executionContext, strategyBotRunApplication.maxConcurrentRounds)
	if findError != nil {
		return 0, findError
	}

	waitGroup := sync.WaitGroup{}
	roundsRun := 0
	roundsRunMutex := sync.Mutex{}

	for index := range dueBots {
		botDto := dueBots[index]

		// A bot already mid-round is skipped rather than queued behind itself. The
		// round waiting would read the same candles and reach the same answer, and
		// arrive as a second message saying it.
		if !strategyBotRunApplication.roundGuard.TryEnter(botDto.ID) {
			continue
		}

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			defer strategyBotRunApplication.roundGuard.Leave(botDto.ID)

			// Each round gets its own deadline rather than sharing the scan's.
			// A round that outlives its own trigger interval has stopped being
			// about now, and one source that will not answer must not be able to
			// hold the whole scan open behind it.
			roundContext, endRound := context.WithTimeout(
				executionContext, strategyBotRunApplication.roundTimeout)
			defer endRound()

			strategyBotRunApplication.runOneRound(roundContext, botDto)

			roundsRunMutex.Lock()
			defer roundsRunMutex.Unlock()
			roundsRun++
		}()
	}

	waitGroup.Wait()

	return roundsRun, nil
}

// runOneRound is one bot's turn, start to finish.
//
// It works out what the round came to and then books it in, in that order and once.
// Splitting the two is what makes it impossible to leave by a path that recorded
// nothing — and a round that records nothing leaves its bot due forever, hammering
// the database and Telegram while looking perfectly healthy from outside.
func (strategyBotRunApplication *StrategyBotRunApplication) runOneRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) {
	outcome := strategyBotRunApplication.playRound(executionContext, botDto)

	_ = strategyBotRunApplication.strategyBotService.RecordRound(
		executionContext, botDto.ID, outcome)
}

// playRound works out what this round comes to, and writes nothing down.
//
// Every return is an outcome, including the failures: what a failure means — wait,
// or stop — was already decided by the failure model, and nothing here decides it
// again.
func (strategyBotRunApplication *StrategyBotRunApplication) playRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) domains.StrategyBotRoundOutcomeDomain {
	signalsByLabel, sourceSignals, roundError := strategyBotRunApplication.readSignals(
		executionContext, botDto)
	if roundError != nil {
		sourceFailure := domains.NewStrategyBotRoundFailureDomain(roundError)
		if sourceFailure.HaltsTheBot() {
			return domains.NewStrategyBotRoundHaltedOutcome(sourceFailure.HaltReason())
		}

		return domains.NewStrategyBotRoundSkippedOutcome()
	}

	decision, decideError := strategyBotRunApplication.strategyBotService.DecideRound(
		botDto, signalsByLabel)
	if decideError != nil {
		// A stored condition that no longer reads is not something time fixes, but
		// it is also not one of the four things a person can go and correct. It
		// waits, on the same rule that covers anything unrecognised: stopping a bot
		// is the destructive answer and is kept for failures that name their cure.
		return domains.NewStrategyBotRoundSkippedOutcome()
	}

	if !decision.ShouldSend {
		return domains.NewStrategyBotRoundConcludedOutcome("", decision.Conflicting)
	}

	deliveryFailure, deliverError := strategyBotRunApplication.sendRoundMessage(
		executionContext, botDto, decision, sourceSignals)
	if deliverError != nil {
		return domains.NewStrategyBotRoundSkippedOutcome()
	}

	failureDomain := domains.NewStrategyBotDeliveryFailureDomain(deliveryFailure)
	if failureDomain.HaltsTheBot() {
		return domains.NewStrategyBotRoundHaltedOutcome(failureDomain.HaltReason())
	}

	// Only a message that arrived counts as said. One Telegram could not take leaves
	// the last sent signal where it was, so the next round offers it again instead of
	// assuming it got through.
	if deliveryFailure != vo.DeliveryFailureNone {
		return domains.NewStrategyBotRoundConcludedOutcome("", decision.Conflicting)
	}

	return domains.NewStrategyBotRoundConcludedOutcome(
		vo.SignalVo(decision.Verdict), decision.Conflicting)
}

// readSignals asks every one of this bot's sources what it says, all at once.
//
// Side by side rather than one after another because they share nothing: each reads
// its own coarseness over its own candles, and ten in sequence would make a round
// take ten times as long as its slowest source for no reason.
//
// The first failure is what comes back. There is no partial round: a condition read
// against some of its sources would be answered against signals that are simply
// absent, and would quietly come out false.
func (strategyBotRunApplication *StrategyBotRunApplication) readSignals(
	executionContext context.Context, botDto dto.StrategyBotDto,
) (map[string]vo.SignalVo, []dto.StrategyBotSourceSignalDto, error) {
	endTime := strategyBotRunApplication.clockProxy.Now()
	startTime := endTime.Add(-strategyBotObservationWindowLength)

	signalsByLabel := map[string]vo.SignalVo{}
	sourceSignals := make([]dto.StrategyBotSourceSignalDto, len(botDto.SignalSources))
	sourceErrors := make([]error, len(botDto.SignalSources))

	waitGroup := sync.WaitGroup{}
	signalsMutex := sync.Mutex{}

	for index := range botDto.SignalSources {
		signalSource := botDto.SignalSources[index]
		resultIndex := index

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			runnableStrategy, resolveError := strategyBotRunApplication.strategyService.ResolveRunnableStrategy(
				executionContext, botDto.OwnerID, signalSource.StrategyID)
			if resolveError != nil {
				sourceErrors[resultIndex] = resolveError

				return
			}

			result, calculateError := strategyBotRunApplication.indicatorCalculationService.CalculateIndicator(
				executionContext, dto.IndicatorCalculationRequestDto{
					Symbol:              botDto.Symbol,
					StartTime:           startTime,
					EndTime:             endTime,
					Script:              runnableStrategy.Script,
					AggregationInterval: signalSource.AggregationInterval,
					// The signal kind is this system's, not the strategy's. A bot
					// reads opinions, so a strategy saved as a number is run for
					// the one thing a bot can use — and refused by the calculation
					// if it cannot produce it.
					ResultType:      string(vo.IndicatorResultTypeSignal),
					Parameters:      runnableStrategy.Parameters,
					ParameterValues: signalSource.ParameterValues,
				})
			if calculateError != nil {
				sourceErrors[resultIndex] = calculateError

				return
			}

			sourceSignals[resultIndex] = dto.StrategyBotSourceSignalDto{
				Label:               signalSource.Label,
				AggregationInterval: signalSource.AggregationInterval,
				Signal:              result.Signal,
			}

			signalsMutex.Lock()
			defer signalsMutex.Unlock()
			signalsByLabel[signalSource.Label] = vo.SignalVo(result.Signal)
		}()
	}

	waitGroup.Wait()

	for _, sourceError := range sourceErrors {
		if sourceError != nil {
			return nil, nil, sourceError
		}
	}

	return signalsByLabel, sourceSignals, nil
}

// sendRoundMessage writes this round out and sends it, and says how that went.
//
// The reference price is read here and a failure to read it is not a failure of the
// round. A message that says the price could not be read is worth more than no
// message: the conclusion is the part somebody acts on, and the price is context.
func (strategyBotRunApplication *StrategyBotRunApplication) sendRoundMessage(
	executionContext context.Context, botDto dto.StrategyBotDto,
	decision dto.StrategyBotRoundDecisionDto, sourceSignals []dto.StrategyBotSourceSignalDto,
) (vo.DeliveryFailureReasonVo, error) {
	round := dto.StrategyBotRoundDto{
		BotName:       botDto.Name,
		Symbol:        botDto.Symbol,
		Verdict:       decision.Verdict,
		SourceSignals: sourceSignals,
	}

	latestCandle, hasLatestCandle, candleError := strategyBotRunApplication.kCandleService.GetLatestKCandle(
		executionContext, botDto.Symbol)
	if candleError == nil && hasLatestCandle {
		round.ReferencePrice = latestCandle.Close
		round.ReferenceTime = latestCandle.OpenTime
		round.HasReference = true
	}

	return strategyBotRunApplication.telegramDeliveryService.SendMessage(
		executionContext, botDto.OwnerID, domains.NewStrategyBotMessageDomain(round).Text())
}
