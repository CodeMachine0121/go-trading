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

// strategyBotObservationWindowLength is how much of the clock one round asks about.
//
// A bot never asks about a stretch; it asks what the market is saying now. One
// minute is the shortest window that holds exactly one slot at every coarseness, so
// whatever the source's interval, the answer is the newest finished candle of it and
// nothing more.
const strategyBotObservationWindowLength = time.Minute

// strategyBotRecordTimeout is how long booking one round in may take.
//
// It is short and fixed because it is three small statements against the database,
// and it is separate from the round's own deadline because it must not inherit time
// the round has already spent — see runOneRound.
const strategyBotRecordTimeout = 15 * time.Second

// StrategyBotRunApplication runs the rounds that are due, and is the only thing that
// does.
//
// It offers one method. Whoever calls it — today a scan on a timer — sequences
// nothing and knows none of the names below: resolving strategy scripts, running scripts
// side by side, reading conditions, sending a message and deciding what a failure
// means all happen behind it. That is deliberate, because everything on that list is
// a step somebody would otherwise be able to leave out.
//
// A spot bot and a contract bot run down the same path. The only two steps that differ
// are reading the market — which calculation a source is asked through, and which
// candle the reference price comes from — and each asks the bot's kind in exactly one
// place. Everything else, from the conditions to the failure rules, is the same for
// both.
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

// RunDueRounds runs one round for each bot that is due, several at a time, and
// reports how many it ran.
//
// The number due is capped by the read rather than by this loop, so a system coming
// back after a long stop does not pull every bot it owns into memory in order to run
// a handful of them. The ones it leaves are due again a minute later.
//
// One bot failing never stops the others: each round keeps its own failure, and a
// bot halted for a deleted strategy script has nothing to do with the bot beside it.
func (strategyBotRunApplication *StrategyBotRunApplication) RunDueRounds(
	executionContext context.Context,
) (int, error) {
	dueBots, findError := strategyBotRunApplication.strategyBotService.FindDueStrategyBots(
		executionContext, strategyBotRunApplication.maxConcurrentRounds)
	if findError != nil {
		return 0, findError
	}

	// A full batch means there were at least this many bots due and some of them
	// are waiting for the next scan. Said out loud because the symptom is otherwise
	// invisible: every bot still reports the interval it asked for, and every bot
	// is quietly running less often than that. Nothing here slows down — the cap is
	// what keeps one scan from pulling every bot it owns into memory — but somebody
	// reading the log can see it is time to raise it.
	if len(dueBots) == strategyBotRunApplication.maxConcurrentRounds {
		log.Printf(
			"strategy bot scan filled its batch of %d; some due bots wait for the next scan",
			strategyBotRunApplication.maxConcurrentRounds)
	}

	waitGroup := sync.WaitGroup{}
	roundsRun := 0
	roundsRunMutex := sync.Mutex{}
	// One round per bot per scan, decided here rather than left to the guard.
	//
	// The guard answers a different question — is this bot mid-round, perhaps from a
	// scan that has not finished yet — and it answers it at the moment it is asked. A
	// batch naming the same bot twice would get past it whenever the first round
	// finished before the loop reached the second entry, which is a matter of how
	// fast the round happened to be. Same bot, same candles, same answer, and a
	// second message saying it.
	scannedBotIDs := map[uint]struct{}{}

	for index := range dueBots {
		botDto := dueBots[index]

		if _, alreadyScanned := scannedBotIDs[botDto.ID]; alreadyScanned {
			continue
		}
		scannedBotIDs[botDto.ID] = struct{}{}

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

			// Each round gets its own deadline rather than sharing the scan's, and
			// that deadline is the shorter of its own trigger interval and the
			// configured ceiling. A round that outlives the interval it belongs to
			// has stopped being about now, and it is still holding one of the few
			// slots the scan has to give.
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

// RunRoundNow runs one round for this person's bot, right now.
//
// It goes down **exactly the same path** as a scheduled round — same signals, same
// conditions, same message, same history — because a button whose answer differs
// from what the bot does on its own is a button that cannot be used to find out what
// the bot does on its own, which is the only reason to have it.
//
// It works on a stopped bot too. Trying one before putting it to work is the
// ordinary way to find out whether it says what you meant, and refusing until it is
// running would mean the only way to test a bot is to leave it running.
func (strategyBotRunApplication *StrategyBotRunApplication) RunRoundNow(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	// Read as this person, so somebody else's bot answers the same "not found" as
	// one that is not there — the same door every other thing about a bot goes
	// through.
	botDto, findError := strategyBotRunApplication.strategyBotService.GetStrategyBot(
		executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, findError
	}

	// The same claim the scan takes. A hand-pressed round and a scheduled one must
	// not both be in flight: they would read the same candles, reach the same
	// answer, and the second would arrive to find the bot already moved on.
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

// runOneRound is one bot's turn, start to finish.
//
// It works out what the round came to and then books it in, in that order and once.
// Splitting the two is what makes it impossible to leave by a path that recorded
// nothing — and a round that records nothing leaves its bot due forever, hammering
// the database and Telegram while looking perfectly healthy from outside.
func (strategyBotRunApplication *StrategyBotRunApplication) runOneRound(
	scanContext context.Context, roundContext context.Context, botDto dto.StrategyBotDto,
) {
	outcomeDto := strategyBotRunApplication.playRound(roundContext, botDto)

	// Booking the round in gets a fresh deadline of its own, taken from the scan
	// rather than from the round. Written on the round's context, this one write
	// would be most likely to fail exactly when the round was slow — and a round
	// that fails to record leaves its bot due forever, re-running and re-sending
	// the same message every scan while the list shows it perfectly healthy.
	// That is the failure this whole split exists to prevent, so it must not
	// depend on time the round has already spent.
	recordContext, endRecord := context.WithTimeout(scanContext, strategyBotRecordTimeout)
	defer endRecord()

	endedBot, applied, recordError := strategyBotRunApplication.strategyBotService.RecordRound(
		recordContext, botDto.ID, botDto.NextRunAt, outcomeDto)
	if recordError != nil {
		// Said out loud rather than swallowed: this is the one failure whose
		// symptom — a hot loop of identical messages — is invisible from the
		// outside, so the log is the only place anybody could ever see it coming.
		log.Printf("strategy bot %d: could not record its round: %v", botDto.ID, recordError)

		return
	}

	// A bot the system just stopped says so, if it still can. Two of the five halt
	// reasons are the message path itself being broken, so those attempts fail —
	// but the other three are exactly the case where somebody would otherwise find
	// out weeks later, by wondering why they had heard nothing.
	//
	// Only when this round was the one that stopped it: a round that arrived to
	// find the bot already moved on has nothing to announce.
	if applied && endedBot.HaltReason != "" {
		_, _ = strategyBotRunApplication.telegramDeliveryService.SendMessage(
			recordContext, endedBot.OwnerID,
			strategyBotRunApplication.strategyBotService.WriteStoppedMessage(endedBot))
	}
}

// roundDeadlineFor is how long this bot's round may take: the shorter of its own
// trigger interval and the configured ceiling.
//
// A one-minute bot allowed two minutes would sit in a single round across two of
// its own intervals, holding a slot the whole time — and it is exactly the slow
// rounds that are most likely to end up unable to book themselves in.
func (strategyBotRunApplication *StrategyBotRunApplication) roundDeadlineFor(
	botDto dto.StrategyBotDto,
) time.Duration {
	triggerInterval := time.Duration(botDto.TriggerIntervalMinutes) * time.Minute
	if triggerInterval > 0 && triggerInterval < strategyBotRunApplication.roundTimeout {
		return triggerInterval
	}

	return strategyBotRunApplication.roundTimeout
}

// playRound works out what this round comes to, and writes nothing down.
//
// Every return is an outcome, including the failures: what a failure means — wait,
// or stop — was already decided by the failure model, and nothing here decides it
// again.
func (strategyBotRunApplication *StrategyBotRunApplication) playRound(
	executionContext context.Context, botDto dto.StrategyBotDto,
) dto.StrategyBotRoundOutcomeDto {
	// Read every round rather than carried on the bot, which is what makes editing
	// a shared set of rules take effect on the next round of every bot following
	// it. It is read as the bot's owner: a round has no signed-in caller, and the
	// clock is not a person.
	tradingStrategyDto, tradingStrategyError := strategyBotRunApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, botDto.OwnerID, botDto.TradingStrategyID)
	if tradingStrategyError != nil {
		// Very nearly unreachable — deleting a set of rules is refused while any bot
		// follows it — but "very nearly" is why this halts rather than skips: a bot
		// with nothing to run that keeps reporting itself as running is the one
		// outcome its owner can neither see nor fix.
		log.Printf("strategy bot %d: round could not read its trading strategy: %v",
			botDto.ID, tradingStrategyError)

		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(tradingStrategyError)
	}

	signalsByLabel, sourceSignals, roundError := strategyBotRunApplication.readSignals(
		executionContext, botDto, tradingStrategyDto)
	if roundError != nil {
		// Said out loud, always. A round that ends here leaves one word in its
		// history — "hold" — which is the same word a round that ran fine and
		// decided nothing leaves. Without this line the two are indistinguishable
		// from outside, and somebody watching a bot say "hold" for a week has no
		// way to find out whether their conditions are wrong or their candles
		// never arrived.
		log.Printf("strategy bot %d: round could not read its signals: %v",
			botDto.ID, roundError)

		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(roundError)
	}

	decision, decideError := strategyBotRunApplication.strategyBotService.DecideRound(
		botDto, tradingStrategyDto, signalsByLabel)
	if decideError != nil {
		log.Printf("strategy bot %d: round could not read its conditions: %v",
			botDto.ID, decideError)

		// A stored condition that no longer reads is not something time fixes, but
		// it is also not one of the four things a person can go and correct. It
		// waits, on the same rule that covers anything unrecognised: stopping a bot
		// is the destructive answer and is kept for failures that name their cure.
		return skippedRound()
	}

	if !decision.ShouldSend {
		// Also said out loud. A bot that concluded something and stayed quiet —
		// because it said the same thing last round, or because its two conditions
		// contradicted each other — looks exactly like a bot that never ran.
		log.Printf("strategy bot %d: round concluded %q and said nothing (last sent %q)",
			botDto.ID, decision.Verdict, botDto.LastSentSignal)

		return concludedRound(decision.Verdict, "", decision.Conflicting)
	}

	// One last look before speaking. Working out this round may have taken a while,
	// and its owner may have deleted the bot in the meantime — a message from a bot
	// somebody just deleted is the one thing a round must never send, because there
	// is no longer anywhere for its owner to go and see where it came from.
	if !strategyBotRunApplication.stillWaitingForThisRound(executionContext, botDto) {
		log.Printf("strategy bot %d: round abandoned — it is no longer waiting for this one",
			botDto.ID)

		return skippedRound()
	}

	positionPlan, hasPositionPlan, deliveryFailure, deliverError := strategyBotRunApplication.sendRoundMessage(
		executionContext, botDto, tradingStrategyDto, decision, sourceSignals)
	if deliverError != nil {
		// Not Telegram refusing — this side failing to ask at all. Most of those
		// are worth waiting out, but one is not: the owner having removed their
		// delivery setting. The same model tells them apart here as everywhere else.
		return strategyBotRunApplication.strategyBotService.ReadRoundFailure(deliverError)
	}

	deliveryOutcome := strategyBotRunApplication.strategyBotService.ReadDeliveryFailure(
		string(deliveryFailure))
	if deliveryOutcome.Kind != roundSkipped {
		return deliveryOutcome
	}

	// Only a message that arrived counts as said. One Telegram could not take leaves
	// the last sent signal where it was, so the next round offers it again instead of
	// assuming it got through.
	//
	// The suggestion is recorded either way. It is what this round worked out, and a
	// history that forgot it because Telegram was down would leave somebody unable to
	// tell a round that suggested nothing from one nobody received.
	if deliveryFailure != vo.DeliveryFailureNone {
		return suggestingRound(
			decision.Verdict, "", decision.Conflicting, positionPlan, hasPositionPlan)
	}

	return suggestingRound(
		decision.Verdict, decision.Verdict, decision.Conflicting,
		positionPlan, hasPositionPlan)
}

// roundSkipped is the outcome kind that changes nothing but when the bot is next
// due. It is named here because this file both produces it and compares against it.
const roundSkipped = "skipped"

// skippedRound and concludedRound build the two outcomes this file produces itself.
// They exist so that no return statement has to spell out which fields belong to
// which kind — that pairing is the one thing about an outcome that can be got wrong.
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

// suggestingRound is a concluded round that also said what to put down, so that the
// history remembers the figures it actually sent.
func suggestingRound(
	verdict string, sentSignal string, conflicting bool,
	positionPlan dto.PositionPlanDto, hasPositionPlan bool,
) dto.StrategyBotRoundOutcomeDto {
	outcomeDto := concludedRound(verdict, sentSignal, conflicting)
	outcomeDto.PositionPlan = positionPlan
	outcomeDto.HasPositionPlan = hasPositionPlan

	return outcomeDto
}

// stillWaitingForThisRound says whether the bot is still there and still waiting for
// the round that is about to speak for it.
//
// A read that itself fails answers no. Staying quiet when this system cannot tell
// whether a bot still exists is the cautious way round: the round is booked in as
// skipped, and the next one says the same thing a minute later.
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
	tradingStrategyDto dto.TradingStrategyDto,
) (map[string]vo.SignalVo, []dto.StrategyBotSourceSignalDto, error) {
	endTime := strategyBotRunApplication.clockProxy.Now()
	startTime := endTime.Add(-strategyBotObservationWindowLength)

	signalsByLabel := map[string]vo.SignalVo{}
	sourceSignals := make([]dto.StrategyBotSourceSignalDto, len(tradingStrategyDto.SignalSources))
	sourceErrors := make([]error, len(tradingStrategyDto.SignalSources))

	waitGroup := sync.WaitGroup{}
	signalsMutex := sync.Mutex{}

	// Which calculation every source is asked through, settled once for the round. A
	// contract bot's sources are contract strategy scripts — the save gate made sure
	// of that — so they are fed the contract bars a contract indicator calculation
	// feeds them, exactly as when that calculation is asked directly.
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
					// A source always speaks in signals. That is settled when the
					// trading strategy is saved — a source may only name a strategy
					// script declared as one — so this is not overriding what the
					// script said; it is the same rule, restated where the script
					// actually runs.
					//
					// Restated rather than read off the source, because a round that
					// began before that rule existed still has to say something a bot
					// can act on, and because a script whose declaration and body
					// disagree should fail on the calculation rather than quietly
					// produce a number nobody can trade on.
					ResultType:      string(vo.IndicatorResultTypeSignal),
					Parameters:      runnableStrategyScript.Parameters,
					ParameterValues: signalSource.ParameterValues,
				})
			if calculateError != nil {
				sourceErrors[resultIndex] = calculateError

				return
			}

			// A contract source that judged by bars no longer arriving has nothing to
			// say about now, and a round built on it is skipped rather than sent.
			if staleError := strategyBotRunApplication.strategyBotService.RequireCurrentSourceReading(
				botDto, result); staleError != nil {
				sourceErrors[resultIndex] = staleError

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
// It also hands back what this round suggested putting down, because the history has
// to remember the figures *this* round used — its owner may well have edited the
// settings by the time anybody reads it back.
func (strategyBotRunApplication *StrategyBotRunApplication) sendRoundMessage(
	executionContext context.Context, botDto dto.StrategyBotDto,
	tradingStrategyDto dto.TradingStrategyDto,
	decision dto.StrategyBotRoundDecisionDto, sourceSignals []dto.StrategyBotSourceSignalDto,
) (dto.PositionPlanDto, bool, vo.DeliveryFailureReasonVo, error) {
	round := dto.StrategyBotRoundDto{
		BotName:        botDto.Name,
		Symbol:         botDto.Symbol,
		MarketDataKind: botDto.MarketDataKind,
		// Read from the rules as they stood when this round read them, which is also
		// what its sources were asked through: what a sell means on this account is
		// part of those rules.
		ContractTradingMode: tradingStrategyDto.TradingMode,
		Verdict:             decision.Verdict,
		// Carried from the bot, not the rules: how much money there is and how much
		// of a move its owner can sit through are facts about this machine. Three
		// bots following one set of rules may each suggest a different size.
		PositionPlanSettings: botDto.PositionPlan,
		SourceSignals:        sourceSignals,
	}

	// The newest one-minute candle of the market this bot eats. A contract bot quotes
	// the contract's last price, not its mark price — the price somebody placing the
	// order sees.
	if botDto.MarketDataKind == string(vo.MarketDataKindContractKCandle) {
		latestContractCandle, hasLatestContractCandle, contractCandleError := strategyBotRunApplication.
			kCandleContractService.GetLatestKCandleContract(executionContext, botDto.Symbol)
		if contractCandleError == nil && hasLatestContractCandle {
			round.ReferencePrice = latestContractCandle.Close
			round.ReferenceTime = latestContractCandle.OpenTime
			round.HasReference = true
		}
	} else {
		latestCandle, hasLatestCandle, candleError := strategyBotRunApplication.kCandleService.GetLatestKCandle(
			executionContext, botDto.Symbol)
		if candleError == nil && hasLatestCandle {
			round.ReferencePrice = latestCandle.Close
			round.ReferenceTime = latestCandle.OpenTime
			round.HasReference = true
		}
	}

	// Worked out once, here, because the same figures reach the message below and the
	// history afterwards. Twice would be two answers, and they would part company the
	// moment somebody edited a setting between the two reads.
	round = strategyBotRunApplication.strategyBotService.PlanRoundPosition(round)

	deliveryFailure, deliverError := strategyBotRunApplication.telegramDeliveryService.SendMessage(
		executionContext,
		botDto.OwnerID,
		strategyBotRunApplication.strategyBotService.WriteRoundMessage(round))

	return round.PositionPlan, round.HasPositionPlan, deliveryFailure, deliverError
}
