package application

import (
	"context"
	"log"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// strategyBotObservationWindowLength is one minute because that is the shortest window
// holding exactly one slot at every coarseness, so a round reads only the newest finished candle.
const strategyBotObservationWindowLength = time.Minute

// strategyBotRecordTimeout is separate from the round's deadline so booking a round in
// never inherits time the round has already spent.
const strategyBotRecordTimeout = 15 * time.Second

// StrategyBotRunApplication is the only thing that runs due rounds; spot and contract bots
// share one path and differ only in how the market is read.
type StrategyBotRunApplication struct {
	strategyBotService                  *service.StrategyBotService
	tradingStrategyService              *service.TradingStrategyService
	strategyScriptService               *service.StrategyScriptService
	indicatorCalculationService         *service.IndicatorCalculationService
	contractIndicatorCalculationService *service.ContractIndicatorCalculationService
	telegramDeliveryService             *service.TelegramDeliveryService
	kCandleService                      *service.KCandleService
	kCandleContractService              *service.KCandleContractService
	clockProxy                          domaininterface.IClockProxy
	roundGuard                          *StrategyBotRoundGuard
	maxConcurrentRounds                 int
	roundTimeout                        time.Duration
}

func NewStrategyBotRunApplication(
	strategyBotService *service.StrategyBotService,
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	indicatorCalculationService *service.IndicatorCalculationService,
	contractIndicatorCalculationService *service.ContractIndicatorCalculationService,
	telegramDeliveryService *service.TelegramDeliveryService,
	kCandleService *service.KCandleService,
	kCandleContractService *service.KCandleContractService,
	clockProxy domaininterface.IClockProxy,
	roundGuard *StrategyBotRoundGuard,
	maxConcurrentRounds int,
	roundTimeout time.Duration,
) *StrategyBotRunApplication {
	return &StrategyBotRunApplication{
		strategyBotService:                  strategyBotService,
		tradingStrategyService:              tradingStrategyService,
		strategyScriptService:               strategyScriptService,
		indicatorCalculationService:         indicatorCalculationService,
		contractIndicatorCalculationService: contractIndicatorCalculationService,
		telegramDeliveryService:             telegramDeliveryService,
		kCandleService:                      kCandleService,
		kCandleContractService:              kCandleContractService,
		clockProxy:                          clockProxy,
		roundGuard:                          roundGuard,
		maxConcurrentRounds:                 maxConcurrentRounds,
		roundTimeout:                        roundTimeout,
	}
}

// RunDueRounds runs one round per due bot concurrently and reports how many ran; the read
// caps the batch, and one bot failing never stops the others.
func (strategyBotRunApplication *StrategyBotRunApplication) RunDueRounds(
	executionContext context.Context,
) (int, error) {
	dueBots, findError := strategyBotRunApplication.strategyBotService.FindDueStrategyBots(
		executionContext, strategyBotRunApplication.maxConcurrentRounds)
	if findError != nil {
		return 0, findError
	}

	// A full batch means some due bots wait for the next scan, which is otherwise invisible.
	if len(dueBots) == strategyBotRunApplication.maxConcurrentRounds {
		log.Printf(
			"strategy bot scan filled its batch of %d; some due bots wait for the next scan",
			strategyBotRunApplication.maxConcurrentRounds)
	}

	waitGroup := sync.WaitGroup{}
	roundsRun := 0
	roundsRunMutex := sync.Mutex{}
	// Deduplicated here because the guard only catches a bot still mid-round, so a batch
	// naming a bot twice would slip past it whenever the first round finished quickly.
	scannedBotIDs := map[uint]struct{}{}

	for index := range dueBots {
		botDto := dueBots[index]

		if _, alreadyScanned := scannedBotIDs[botDto.ID]; alreadyScanned {
			continue
		}
		scannedBotIDs[botDto.ID] = struct{}{}

		if !strategyBotRunApplication.roundGuard.TryEnter(botDto.ID) {
			continue
		}

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			defer strategyBotRunApplication.roundGuard.Leave(botDto.ID)

			roundContext, endRound := context.WithTimeout(
				executionContext, strategyBotRunApplication.roundDeadlineFor(botDto))
			defer endRound()

			strategyBotRunApplication.runOneRound(executionContext, roundContext, botDto)

			roundsRunMutex.Lock()
			defer roundsRunMutex.Unlock()
			roundsRun++
		}()
	}

	waitGroup.Wait()

	return roundsRun, nil
}

// RunRoundNow runs this person's bot through exactly the same path as a scheduled round,
// including when the bot is stopped, so it can be used to test what the bot would do.
func (strategyBotRunApplication *StrategyBotRunApplication) RunRoundNow(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	// Read as this person so somebody else's bot answers "not found".
	botDto, findError := strategyBotRunApplication.strategyBotService.GetStrategyBot(
		executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, findError
	}

	// Shares the scan's claim so a hand-pressed and a scheduled round are never in flight together.
	if !strategyBotRunApplication.roundGuard.TryEnter(id) {
		return dto.StrategyBotDto{}, domains.StrategyBotAlreadyRunningARound()
	}
	defer strategyBotRunApplication.roundGuard.Leave(id)

	roundContext, endRound := context.WithTimeout(
		executionContext, strategyBotRunApplication.roundDeadlineFor(botDto))
	defer endRound()

	strategyBotRunApplication.runOneRound(executionContext, roundContext, botDto)

	return strategyBotRunApplication.strategyBotService.GetStrategyBot(
		executionContext, viewerID, id)
}

// runOneRound works out the round's outcome and then always books it in, because a round
// that records nothing leaves its bot due forever.
func (strategyBotRunApplication *StrategyBotRunApplication) runOneRound(
	scanContext context.Context, roundContext context.Context, botDto dto.StrategyBotDto,
) {
	outcomeDto := strategyBotRunApplication.playRound(roundContext, botDto)

	// Derived from the scan context, not the round's, so a slow round cannot starve the one
	// write that keeps its bot from re-running and re-sending every scan.
	recordContext, endRecord := context.WithTimeout(scanContext, strategyBotRecordTimeout)
	defer endRecord()

	endedBot, applied, recordError := strategyBotRunApplication.strategyBotService.RecordRound(
		recordContext, botDto.ID, botDto.NextRunAt, outcomeDto)
	if recordError != nil {
		log.Printf("strategy bot %d: could not record its round: %v", botDto.ID, recordError)

		return
	}

	// Announce a halt only when this round caused it; the attempt fails harmlessly when the
	// halt reason is the message path itself being broken.
	if applied && endedBot.HaltReason != "" {
		_, _ = strategyBotRunApplication.telegramDeliveryService.SendMessage(
			recordContext, endedBot.OwnerID,
			strategyBotRunApplication.strategyBotService.WriteStoppedMessage(endedBot))
	}
}

// roundDeadlineFor is the shorter of the bot's trigger interval and the configured ceiling,
// so a round never spans more than one of its own intervals.
func (strategyBotRunApplication *StrategyBotRunApplication) roundDeadlineFor(
	botDto dto.StrategyBotDto,
) time.Duration {
	triggerInterval := time.Duration(botDto.TriggerIntervalMinutes) * time.Minute
	if triggerInterval > 0 && triggerInterval < strategyBotRunApplication.roundTimeout {
		return triggerInterval
	}

	return strategyBotRunApplication.roundTimeout
}

// playRound works out what this round comes to without writing anything; failures are
// outcomes too, already classified by the failure model.
func (strategyBotRunApplication *StrategyBotRunApplication) playRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) dto.StrategyBotRoundOutcomeDto {
	// Read every round, as the bot's owner, so edits to shared rules apply on the next round.
	tradingStrategyDto, tradingStrategyError := strategyBotRunApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, botDto.OwnerID, botDto.TradingStrategyID)
	if tradingStrategyError != nil {
		// Nearly unreachable since deleting a followed strategy is refused, but it halts
		// rather than skips so the bot does not silently report itself as running.
		log.Printf("strategy bot %d: round could not read its trading strategy: %v",
			botDto.ID, tradingStrategyError)

		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(tradingStrategyError)
	}

	// A contract bot reads its newest candle up front: it is both the staleness check (contracts
	// trade round the clock) and the last-price reference; spot markets close, so spot reads it later.
	reference := dto.StrategyBotRoundDto{}
	if botDto.MarketDataKind == string(vo.MarketDataKindContractKCandle) {
		latestContractCandle, hasLatestContractCandle, contractCandleError := strategyBotRunApplication.
			kCandleContractService.GetLatestKCandleContract(executionContext, botDto.Symbol)
		if contractCandleError == nil && hasLatestContractCandle {
			reference.ReferencePrice = latestContractCandle.Close
			reference.ReferenceTime = latestContractCandle.OpenTime
			reference.HasReference = true
		}

		if staleError := strategyBotRunApplication.strategyBotService.RequireCurrentMarket(
			botDto, reference.ReferenceTime, reference.HasReference); staleError != nil {
			log.Printf("strategy bot %d: round skipped: %v", botDto.ID, staleError)

			return strategyBotRunApplication.strategyBotService.ReadRoundFailure(staleError)
		}
	}

	signalsByLabel, sourceSignals, roundError := strategyBotRunApplication.readSignals(
		executionContext, botDto, tradingStrategyDto)
	if roundError != nil {
		// Logged because the history records "hold" either way, indistinguishable from a
		// healthy round that concluded nothing.
		log.Printf("strategy bot %d: round could not read its signals: %v",
			botDto.ID, roundError)

		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(roundError)
	}

	decision, decideError := strategyBotRunApplication.strategyBotService.DecideRound(
		botDto, tradingStrategyDto, signalsByLabel)
	if decideError != nil {
		log.Printf("strategy bot %d: round could not read its conditions: %v",
			botDto.ID, decideError)

		// An unreadable stored condition waits rather than halts: stopping is reserved for
		// failures a person can correct.
		return skippedRound()
	}

	if !decision.ShouldSend {
		log.Printf("strategy bot %d: round concluded %q and said nothing (last sent %q)",
			botDto.ID, decision.Verdict, botDto.LastSentSignal)

		return concludedRound(decision.Verdict, "", decision.Conflicting)
	}

	// The owner may have deleted the bot while the round was working; never send for a deleted bot.
	if !strategyBotRunApplication.stillWaitingForThisRound(executionContext, botDto) {
		log.Printf("strategy bot %d: round abandoned — it is no longer waiting for this one",
			botDto.ID)

		return skippedRound()
	}

	positionPlan, hasPositionPlan, deliveryFailure, deliverError := strategyBotRunApplication.sendRoundMessage(
		executionContext, botDto, tradingStrategyDto, reference, decision, sourceSignals)
	if deliverError != nil {
		// This side failing to ask (not Telegram refusing), e.g. the owner removed their delivery setting.
		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(deliverError)
	}

	deliveryOutcome := strategyBotRunApplication.strategyBotService.ReadDeliveryFailure(
		string(deliveryFailure))
	if deliveryOutcome.Kind != roundSkipped {
		return deliveryOutcome
	}

	// An undelivered message leaves the last sent signal unchanged so the next round retries,
	// but the suggestion is still recorded in history.
	if deliveryFailure != vo.DeliveryFailureNone {
		return suggestingRound(
			decision.Verdict, "", decision.Conflicting, positionPlan, hasPositionPlan)
	}

	return suggestingRound(
		decision.Verdict, decision.Verdict, decision.Conflicting,
		positionPlan, hasPositionPlan)
}

// roundSkipped is the outcome kind that changes nothing but when the bot is next due.
const roundSkipped = "skipped"

// skippedRound and concludedRound keep the pairing of kind and fields in one place.
func skippedRound() dto.StrategyBotRoundOutcomeDto {
	return dto.StrategyBotRoundOutcomeDto{Kind: roundSkipped}
}

func concludedRound(verdict string, sentSignal string, conflicting bool) dto.StrategyBotRoundOutcomeDto {
	return dto.StrategyBotRoundOutcomeDto{
		Kind:        "concluded",
		Verdict:     verdict,
		SentSignal:  sentSignal,
		Conflicting: conflicting,
	}
}

// suggestingRound is a concluded round that also records the position plan it sent.
func suggestingRound(
	verdict string, sentSignal string, conflicting bool,
	positionPlan dto.PositionPlanDto, hasPositionPlan bool,
) dto.StrategyBotRoundOutcomeDto {
	outcomeDto := concludedRound(verdict, sentSignal, conflicting)
	outcomeDto.PositionPlan = positionPlan
	outcomeDto.HasPositionPlan = hasPositionPlan

	return outcomeDto
}

// stillWaitingForThisRound reports whether the bot still exists and still awaits this round;
// a failed read answers no, erring toward staying quiet.
func (strategyBotRunApplication *StrategyBotRunApplication) stillWaitingForThisRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) bool {
	current, findError := strategyBotRunApplication.strategyBotService.GetStrategyBot(
		executionContext, botDto.OwnerID, botDto.ID)
	if findError != nil {
		return false
	}

	return current.NextRunAt.Equal(botDto.NextRunAt)
}

// readSignals asks every source concurrently and returns the first failure, since a
// condition read against partial signals would quietly come out false.
func (strategyBotRunApplication *StrategyBotRunApplication) readSignals(
	executionContext context.Context, botDto dto.StrategyBotDto,
	tradingStrategyDto dto.TradingStrategyDto,
) (map[string]vo.SignalVo, []dto.StrategyBotSourceSignalDto, error) {
	endTime := strategyBotRunApplication.clockProxy.Now()
	startTime := endTime.Add(-strategyBotObservationWindowLength)

	signalsByLabel := map[string]vo.SignalVo{}
	sourceSignals := make([]dto.StrategyBotSourceSignalDto, len(tradingStrategyDto.SignalSources))
	sourceErrors := make([]error, len(tradingStrategyDto.SignalSources))

	waitGroup := sync.WaitGroup{}
	signalsMutex := sync.Mutex{}

	// A contract bot's sources are contract scripts (enforced on save), so they are fed contract bars.
	calculateSignal := strategyBotRunApplication.indicatorCalculationService.CalculateIndicator
	if botDto.MarketDataKind == string(vo.MarketDataKindContractKCandle) {
		calculateSignal = strategyBotRunApplication.contractIndicatorCalculationService.CalculateContractIndicator
	}

	for index := range tradingStrategyDto.SignalSources {
		signalSource := tradingStrategyDto.SignalSources[index]
		resultIndex := index

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			runnableStrategyScript, resolveError := strategyBotRunApplication.strategyScriptService.ResolveRunnableStrategyScript(
				executionContext, botDto.OwnerID, signalSource.StrategyScriptID)
			if resolveError != nil {
				sourceErrors[resultIndex] = resolveError

				return
			}

			result, calculateError := calculateSignal(
				executionContext, dto.IndicatorCalculationRequestDto{
					Symbol:              botDto.Symbol,
					StartTime:           startTime,
					EndTime:             endTime,
					Script:              runnableStrategyScript.Script,
					AggregationInterval: signalSource.AggregationInterval,
					// Forced to signal (restating the save-time rule) so a script whose body
					// disagrees with its declaration fails here instead of yielding an untradeable number.
					ResultType:      string(vo.IndicatorResultTypeSignal),
					Parameters:      runnableStrategyScript.Parameters,
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

// sendRoundMessage sends the round's message and returns the position plan it used; an
// unreadable reference price still sends, since the conclusion matters more than the price.
func (strategyBotRunApplication *StrategyBotRunApplication) sendRoundMessage(
	executionContext context.Context, botDto dto.StrategyBotDto,
	tradingStrategyDto dto.TradingStrategyDto, reference dto.StrategyBotRoundDto,
	decision dto.StrategyBotRoundDecisionDto, sourceSignals []dto.StrategyBotSourceSignalDto,
) (dto.PositionPlanDto, bool, vo.DeliveryFailureReasonVo, error) {
	round := dto.StrategyBotRoundDto{
		BotName:             botDto.Name,
		Symbol:              botDto.Symbol,
		MarketDataKind:      botDto.MarketDataKind,
		ContractTradingMode: tradingStrategyDto.TradingMode,
		Verdict:             decision.Verdict,
		ReferencePrice:      reference.ReferencePrice,
		ReferenceTime:       reference.ReferenceTime,
		HasReference:        reference.HasReference,
		// From the bot, not the rules: capital and risk tolerance differ per bot sharing one strategy.
		PositionPlanSettings: botDto.PositionPlan,
		SourceSignals:        sourceSignals,
	}

	if botDto.MarketDataKind != string(vo.MarketDataKindContractKCandle) {
		latestCandle, hasLatestCandle, candleError := strategyBotRunApplication.kCandleService.GetLatestKCandle(
			executionContext, botDto.Symbol)
		if candleError == nil && hasLatestCandle {
			round.ReferencePrice = latestCandle.Close
			round.ReferenceTime = latestCandle.OpenTime
			round.HasReference = true
		}
	}

	// Planned once so the message and the history carry identical figures.
	round = strategyBotRunApplication.strategyBotService.PlanRoundPosition(executionContext, round)

	deliveryFailure, deliverError := strategyBotRunApplication.telegramDeliveryService.SendMessage(
		executionContext,
		botDto.OwnerID,
		strategyBotRunApplication.strategyBotService.WriteRoundMessage(round))

	return round.PositionPlan, round.HasPositionPlan, deliveryFailure, deliverError
}
