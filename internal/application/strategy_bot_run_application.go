package application

import (
	"context"
	"log"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
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
	kCandleService                      *service.KCandleService
	kCandleContractService              *service.KCandleContractService
	tradeJournalLinkService             *service.TradeJournalLinkService
	clockProxy                          domaininterface.IClockProxy
	// replicaName is who this replica's round claims say is running the bot.
	replicaName         string
	maxConcurrentRounds int
	roundTimeout        time.Duration
	// orderFlowPreparationAllowance bounds gathering footprints within a round, so a slow first gather skips the round instead of overrunning it.
	orderFlowPreparationAllowance time.Duration
}

func NewStrategyBotRunApplication(
	strategyBotService *service.StrategyBotService,
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	indicatorCalculationService *service.IndicatorCalculationService,
	contractIndicatorCalculationService *service.ContractIndicatorCalculationService,
	kCandleService *service.KCandleService,
	kCandleContractService *service.KCandleContractService,
	tradeJournalLinkService *service.TradeJournalLinkService,
	clockProxy domaininterface.IClockProxy,
	replicaName string,
	maxConcurrentRounds int,
	roundTimeout time.Duration,
	orderFlowPreparationAllowance time.Duration,
) *StrategyBotRunApplication {
	return &StrategyBotRunApplication{
		strategyBotService:                  strategyBotService,
		tradingStrategyService:              tradingStrategyService,
		strategyScriptService:               strategyScriptService,
		indicatorCalculationService:         indicatorCalculationService,
		contractIndicatorCalculationService: contractIndicatorCalculationService,
		kCandleService:                      kCandleService,
		kCandleContractService:              kCandleContractService,
		tradeJournalLinkService:             tradeJournalLinkService,
		clockProxy:                          clockProxy,
		replicaName:                         replicaName,
		maxConcurrentRounds:                 maxConcurrentRounds,
		roundTimeout:                        roundTimeout,
		orderFlowPreparationAllowance:       orderFlowPreparationAllowance,
	}
}

// RunDueRounds runs one round per due bot concurrently and reports how many ran; the read
// caps the batch, and one bot failing never stops the others.
func (strategyBotRunApplication *StrategyBotRunApplication) RunDueRounds(
	executionContext context.Context,
) (int, error) {
	// Claimed rather than just read, so no other replica runs these bots until this one is done with them.
	dueBots, claimError := strategyBotRunApplication.strategyBotService.ClaimDueStrategyBots(
		executionContext, strategyBotRunApplication.maxConcurrentRounds,
		strategyBotRunApplication.replicaName, strategyBotRunApplication.roundClaimDuration())
	if claimError != nil {
		return 0, claimError
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

	for index := range dueBots {
		botDto := dueBots[index]

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

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

	// Shares the scan's claim so a hand-pressed and a scheduled round are never in flight together, on any replica.
	if claimError := strategyBotRunApplication.strategyBotService.ClaimStrategyBot(
		executionContext, id, strategyBotRunApplication.replicaName,
		strategyBotRunApplication.roundClaimDuration()); claimError != nil {
		return dto.StrategyBotDto{}, claimError
	}

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

	// Its own deadline, not the round's and not the caller's: a slow round or a hand-pressed request
	// the caller gave up on must not cancel the one write that books the round and frees its claim.
	recordContext, endRecord := context.WithTimeout(
		context.WithoutCancel(scanContext), strategyBotRecordTimeout)
	defer endRecord()

	// The round's message and any halt notice are queued in the same write, so nothing is said here.
	_, _, recordError := strategyBotRunApplication.strategyBotService.RecordRound(
		recordContext, botDto.ID, botDto.NextRunAt, strategyBotRunApplication.replicaName, outcomeDto)
	if recordError == nil {
		return
	}
	log.Printf("strategy bot %d: could not record its round: %v", botDto.ID, recordError)

	// The release rolled back with the booking; freed on its own so the bot is not kept from every replica until the claim runs out.
	if releaseError := strategyBotRunApplication.strategyBotService.ReleaseStrategyBotClaim(
		recordContext, botDto.ID, strategyBotRunApplication.replicaName); releaseError != nil {
		log.Printf("strategy bot %d: could not free its claim: %v", botDto.ID, releaseError)
	}
}

// roundClaimDuration outlasts the longest a round may take plus booking it in, so a live round's claim never lapses.
func (strategyBotRunApplication *StrategyBotRunApplication) roundClaimDuration() time.Duration {
	return strategyBotRunApplication.roundTimeout + strategyBotRecordTimeout
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

// playRound works out what this round comes to on every watched symbol not halted on its own, without writing anything;
// failures are outcomes too, already classified by the failure model.
func (strategyBotRunApplication *StrategyBotRunApplication) playRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) dto.StrategyBotRoundOutcomeDto {
	watchedSymbolsToPlay := strategyBotRunApplication.strategyBotService.WatchedSymbolsToPlay(botDto)
	watchedSymbolOutcomes := make([]dto.StrategyBotWatchedSymbolRoundOutcomeDto, len(watchedSymbolsToPlay))

	// The rules and their scripts are read once per round, as the bot's owner, so every symbol follows the same version and edits apply on the next round.
	// A trading strategy gone is nearly unreachable since deleting a followed one is refused, but like a script gone it halts rather than skips, so the bot does not silently report itself as running.
	tradingStrategyDto, roundError := strategyBotRunApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, botDto.OwnerID, botDto.TradingStrategyID)
	runnableStrategyScripts := make([]dto.RunnableStrategyScriptDto, len(tradingStrategyDto.SignalSources))
	for index, signalSource := range tradingStrategyDto.SignalSources {
		if roundError != nil {
			break
		}
		runnableStrategyScripts[index], roundError = strategyBotRunApplication.strategyScriptService.ResolveOwnedStrategyScript(
			executionContext, botDto.OwnerID, signalSource.StrategyScriptID)
	}
	if roundError != nil {
		log.Printf("strategy bot %d: round could not read its rules: %v", botDto.ID, roundError)

		for index, watchedSymbolDto := range watchedSymbolsToPlay {
			watchedSymbolOutcomes[index] = strategyBotRunApplication.strategyBotService.ReadRoundFailure(
				watchedSymbolDto.Symbol, roundError)
		}

		return dto.StrategyBotRoundOutcomeDto{WatchedSymbolOutcomes: watchedSymbolOutcomes}
	}

	// Symbols run side by side within the round's one deadline; each lands in its own slot, so the bot's order is kept.
	waitGroup := sync.WaitGroup{}
	for index, watchedSymbolDto := range watchedSymbolsToPlay {
		waitGroup.Go(func() {
			watchedSymbolOutcomes[index] = strategyBotRunApplication.playWatchedSymbolRound(
				executionContext, botDto, watchedSymbolDto, tradingStrategyDto, runnableStrategyScripts)
		})
	}
	waitGroup.Wait()

	return dto.StrategyBotRoundOutcomeDto{WatchedSymbolOutcomes: watchedSymbolOutcomes}
}

// playWatchedSymbolRound works out what this round comes to on one watched symbol.
func (strategyBotRunApplication *StrategyBotRunApplication) playWatchedSymbolRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
	watchedSymbolDto dto.StrategyBotWatchedSymbolDto, tradingStrategyDto dto.TradingStrategyDto,
	runnableStrategyScripts []dto.RunnableStrategyScriptDto,
) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	symbol := watchedSymbolDto.Symbol

	// A contract bot reads its newest candle up front: it is both the staleness check (contracts
	// trade round the clock) and the last-price reference; spot markets close, so spot reads it later.
	reference := dto.StrategyBotRoundDto{}
	if botDto.MarketDataKind == string(vo.MarketDataKindContractKCandle) {
		latestContractCandle, hasLatestContractCandle, contractCandleError := strategyBotRunApplication.
			kCandleContractService.GetLatestKCandleContract(executionContext, symbol)
		if contractCandleError == nil && hasLatestContractCandle {
			reference.ReferencePrice = latestContractCandle.Close
			reference.ReferenceTime = latestContractCandle.OpenTime
			reference.HasReference = true
		}

		if staleError := strategyBotRunApplication.strategyBotService.RequireCurrentMarket(
			botDto, reference.ReferenceTime, reference.HasReference); staleError != nil {
			log.Printf("strategy bot %d on %s: round skipped: %v", botDto.ID, symbol, staleError)

			return strategyBotRunApplication.strategyBotService.ReadRoundFailure(symbol, staleError)
		}
	}

	signalsByLabel, sourceSignals, roundError := strategyBotRunApplication.readSignals(
		executionContext, botDto, symbol, tradingStrategyDto, runnableStrategyScripts)
	if roundError != nil {
		// Logged because the history records "hold" either way, indistinguishable from a
		// healthy round that concluded nothing.
		log.Printf("strategy bot %d on %s: round could not read its signals: %v",
			botDto.ID, symbol, roundError)

		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(symbol, roundError)
	}

	decision, decideError := strategyBotRunApplication.strategyBotService.DecideRound(
		watchedSymbolDto, tradingStrategyDto, signalsByLabel)
	if decideError != nil {
		log.Printf("strategy bot %d on %s: round could not read its conditions: %v",
			botDto.ID, symbol, decideError)

		// An unreadable stored condition waits rather than halts: stopping is reserved for
		// failures a person can correct.
		return skippedRound(symbol)
	}

	if !decision.ShouldSend {
		log.Printf("strategy bot %d on %s: round concluded %q and said nothing (last sent %q)",
			botDto.ID, symbol, decision.Verdict, watchedSymbolDto.LastSentSignal)

		return concludedRound(symbol, decision.Verdict, "", decision.Conflicting)
	}

	// A bot deleted or restarted meanwhile is caught when the round is booked in, before anything is queued.
	suggestedRound := strategyBotRunApplication.composeRoundMessage(
		executionContext, botDto, watchedSymbolDto, tradingStrategyDto, reference, decision, sourceSignals)

	outcomeDto := suggestingRound(symbol, decision.Verdict, decision.Conflicting, suggestedRound)
	outcomeDto.AutoOrderIntent, outcomeDto.HasAutoOrderIntent = strategyBotRunApplication.strategyBotService.
		PlanAutoOrderIntent(suggestedRound)

	return outcomeDto
}

// roundSkipped is the outcome kind that changes nothing but when the bot is next due.
const roundSkipped = "skipped"

// skippedRound and concludedRound keep the pairing of kind and fields in one place.
func skippedRound(symbol string) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	return dto.StrategyBotWatchedSymbolRoundOutcomeDto{Symbol: symbol, Kind: roundSkipped}
}

func concludedRound(
	symbol string, verdict string, saidSignal string, conflicting bool,
) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	return dto.StrategyBotWatchedSymbolRoundOutcomeDto{
		Symbol:      symbol,
		Kind:        "concluded",
		Verdict:     verdict,
		SentSignal:  saidSignal,
		Conflicting: conflicting,
	}
}

// suggestingRound is a concluded round that says its verdict: it carries the message to queue, the position plan it suggests and, with a journal link, the reference price the link prefills.
func suggestingRound(
	symbol string, verdict string, conflicting bool, suggestedRound dto.StrategyBotRoundDto,
) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	outcomeDto := concludedRound(symbol, verdict, verdict, conflicting)
	outcomeDto.Round = suggestedRound
	outcomeDto.HasMessage = true
	outcomeDto.PositionPlan = suggestedRound.PositionPlan
	outcomeDto.HasPositionPlan = suggestedRound.HasPositionPlan

	if suggestedRound.JournalLinkIdentifier != "" {
		outcomeDto.JournalLinkIdentifier = suggestedRound.JournalLinkIdentifier
		outcomeDto.ReferencePrice = decimal.NullDecimal{Decimal: suggestedRound.ReferencePrice, Valid: suggestedRound.HasReference}
	}

	return outcomeDto
}

// readSignals asks every source concurrently about one symbol, running the scripts the round resolved for them and returns the first failure, since a
// condition read against partial signals would quietly come out false.
func (strategyBotRunApplication *StrategyBotRunApplication) readSignals(
	executionContext context.Context, botDto dto.StrategyBotDto, symbol string,
	tradingStrategyDto dto.TradingStrategyDto, runnableStrategyScripts []dto.RunnableStrategyScriptDto,
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

			runnableStrategyScript := runnableStrategyScripts[resultIndex]
			result, calculateError := calculateSignal(
				executionContext, dto.IndicatorCalculationRequestDto{
					Symbol:              symbol,
					StartTime:           startTime,
					EndTime:             endTime,
					Script:              runnableStrategyScript.Script,
					AggregationInterval: signalSource.AggregationInterval,
					// Forced to signal (restating the save-time rule) so a script whose body
					// disagrees with its declaration fails here instead of yielding an untradeable number.
					ResultType:      string(vo.IndicatorResultTypeSignal),
					Parameters:      runnableStrategyScript.Parameters,
					ParameterValues: signalSource.ParameterValues,
					// Only contract scripts may declare order flow, so a spot calculation never reads it.
					OrderFlowPreparationAllowance: strategyBotRunApplication.orderFlowPreparationAllowance,
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

// composeRoundMessage works out everything the round's message says; an unreadable reference
// price still yields a message, since the conclusion matters more than the price.
func (strategyBotRunApplication *StrategyBotRunApplication) composeRoundMessage(
	executionContext context.Context, botDto dto.StrategyBotDto, watchedSymbolDto dto.StrategyBotWatchedSymbolDto,
	tradingStrategyDto dto.TradingStrategyDto, reference dto.StrategyBotRoundDto,
	decision dto.StrategyBotRoundDecisionDto, sourceSignals []dto.StrategyBotSourceSignalDto,
) dto.StrategyBotRoundDto {
	symbol := watchedSymbolDto.Symbol
	round := dto.StrategyBotRoundDto{
		BotName:             botDto.Name,
		Symbol:              symbol,
		MarketDataKind:      botDto.MarketDataKind,
		ContractTradingMode: tradingStrategyDto.TradingMode,
		Verdict:             decision.Verdict,
		ReferencePrice:      reference.ReferencePrice,
		ReferenceTime:       reference.ReferenceTime,
		HasReference:        reference.HasReference,
		// From the bot, not the rules: capital and risk tolerance differ per bot sharing one strategy.
		PositionPlanSettings: botDto.PositionPlan,
		AllocationPercentage: watchedSymbolDto.AllocationPercentage,
		SourceSignals:        sourceSignals,
	}

	if botDto.MarketDataKind != string(vo.MarketDataKindContractKCandle) {
		latestCandle, hasLatestCandle, candleError := strategyBotRunApplication.kCandleService.GetLatestKCandle(
			executionContext, symbol)
		if candleError == nil && hasLatestCandle {
			round.ReferencePrice = latestCandle.Close
			round.ReferenceTime = latestCandle.OpenTime
			round.HasReference = true
		}
	}

	// Planned once so the message and the history carry identical figures.
	round = strategyBotRunApplication.strategyBotService.PlanRoundPosition(executionContext, round)
	return strategyBotRunApplication.tradeJournalLinkService.OfferJournalLink(round)
}
