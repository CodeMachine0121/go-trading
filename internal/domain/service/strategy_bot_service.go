package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotService is the application layer's only entry point for strategy bots.
// Its public use-case methods never call one another.
//
// It is given no way to reach a strategy script, a K candle or Telegram, and that is the
// point. Whether a bot may name a strategy script is the strategy script rules' question; whether
// its owner can be spoken to is the delivery setting's; and joining those to this is
// the application layer's job. A dependency that is not here cannot be reached for
// by accident later.
//
// The three contract readers are the exception, and a narrow one: which contract a
// contract bot may watch and how much leverage it may carry there, read when a bot is
// saved; and the venue's rules and latest funding a contract suggestion is worked out
// by, read only in a round that has a suggestion to make.
type StrategyBotService struct {
	strategyBotRepository                   domaininterface.IStrategyBotRepository
	strategyBotRunRecordRepository          domaininterface.IStrategyBotRunRecordRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	clockProxy                              domaininterface.IClockProxy
}

func NewStrategyBotService(
	strategyBotRepository domaininterface.IStrategyBotRepository,
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	clockProxy domaininterface.IClockProxy,
) *StrategyBotService {
	return &StrategyBotService{
		strategyBotRepository:                   strategyBotRepository,
		strategyBotRunRecordRepository:          strategyBotRunRecordRepository,
		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository: contractMaintenanceMarginTierRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		clockProxy:                              clockProxy,
	}
}

// CreateStrategyBot saves a new bot for its owner, stopped, and hands it back as
// stored. A bot that breaks a rule is refused before anything is written.
//
// The trading strategy it follows arrives already read, as the caller could see it:
// whether it is theirs to follow was answered there, and what is asked of it here is
// only whether it eats the same kind of market as the bot.
func (strategyBotService *StrategyBotService) CreateStrategyBot(
	executionContext context.Context, writeDto dto.StrategyBotWriteDto,
	followedTradingStrategy dto.TradingStrategyDto,
) (dto.StrategyBotDto, error) {
	// An identifier arriving on a create would rewrite whichever bot it named,
	// including somebody else's. Clearing it makes creating unable to mean that.
	writeDto.ID = 0

	strategyBotDomain, validationError := strategyBotService.settle(
		executionContext, writeDto, followedTradingStrategy)
	if validationError != nil {
		return dto.StrategyBotDto{}, validationError
	}

	savedBot, saveError := strategyBotService.strategyBotRepository.Save(
		executionContext, strategyBotDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyBotDto{}, saveError
	}

	return savedBot.ToDto(), nil
}

// ListStrategyBots returns this person's bots, by name — all of them, or only those of
// one kind of market when one is named.
//
// Having none is an empty list rather than a refusal: it is the ordinary state of
// somebody who has not built one yet. Naming a kind nobody recognises is refused,
// rather than answered with nothing: an empty list would read as "you have none".
func (strategyBotService *StrategyBotService) ListStrategyBots(
	executionContext context.Context, ownerID uint, marketDataKind string,
) ([]dto.StrategyBotDto, error) {
	narrowsToKind := strings.TrimSpace(marketDataKind) != ""

	wantedKind, kindError := domains.NewMarketDataKindDomain(marketDataKind)
	if kindError != nil {
		return nil, fmt.Errorf("%w: %w", domains.ErrStrategyBotValidation, kindError)
	}

	bots, findError := strategyBotService.strategyBotRepository.FindAllByOwner(
		executionContext, ownerID)
	if findError != nil {
		return nil, findError
	}

	botDtos := make([]dto.StrategyBotDto, 0, len(bots))
	for _, bot := range bots {
		botDto := bot.ToDto()
		if narrowsToKind && botDto.MarketDataKind != string(wantedKind.Value()) {
			continue
		}

		botDtos = append(botDtos, botDto)
	}

	return botDtos, nil
}

// GetStrategyBot returns the viewer's own bot carrying this identifier.
func (strategyBotService *StrategyBotService) GetStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	bot, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, findError
	}

	return bot.ToDto(), nil
}

// UpdateStrategyBot rewrites the viewer's own bot, and refuses while it is running.
//
// Every rule that applied to creating it applies here word for word, because both
// arrive as the same shape and are checked by the same model — plus one of its own:
// the kind of market it eats is the one it was created with.
func (strategyBotService *StrategyBotService) UpdateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
	followedTradingStrategy dto.TradingStrategyDto,
) (dto.StrategyBotDto, error) {
	storedBot, findError := strategyBotService.findOwnedBot(
		executionContext, viewerID, writeDto.ID)
	if findError != nil {
		return dto.StrategyBotDto{}, findError
	}

	if editableError := domains.NewStrategyBotRunStateDomain(storedBot).RequireEditable(); editableError != nil {
		return dto.StrategyBotDto{}, editableError
	}

	// The owner comes from what is stored, never from what arrived. A bot cannot
	// change hands, and the write path not being able to say so is stronger than
	// remembering not to.
	writeDto.OwnerID = storedBot.OwnerID

	// The stored kind is read the way every reader reads it, so a bot stored before
	// there was a choice keeps the K candle it has always eaten.
	storedKind, storedKindError := domains.NewMarketDataKindDomain(storedBot.MarketDataKind)
	if storedKindError != nil {
		return dto.StrategyBotDto{}, fmt.Errorf("%w: %w", domains.ErrStrategyBotValidation, storedKindError)
	}

	retainedKind, retainError := storedKind.RetainingForStrategyBot(writeDto.MarketDataKind)
	if retainError != nil {
		return dto.StrategyBotDto{}, retainError
	}
	writeDto.MarketDataKind = string(retainedKind.Value())

	strategyBotDomain, validationError := strategyBotService.settle(
		executionContext, writeDto, followedTradingStrategy)
	if validationError != nil {
		return dto.StrategyBotDto{}, validationError
	}

	savedBot, saveError := strategyBotService.strategyBotRepository.Save(
		executionContext, strategyBotDomain.ToEntity())
	if saveError != nil {
		return dto.StrategyBotDto{}, saveError
	}

	return savedBot.ToDto(), nil
}

// DeleteStrategyBot removes the viewer's own bot, running or not.
//
// A running bot is not refused here. What was asked for is that this bot stop
// existing, and something that does not exist is never picked up again — making them
// press stop first would only be a second button between them and the same outcome.
func (strategyBotService *StrategyBotService) DeleteStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id); findError != nil {
		return findError
	}

	return strategyBotService.strategyBotRepository.Delete(executionContext, id)
}

// ReadReferencesTo says who is following this set of rules: how many bots in total,
// and which of them are running.
//
// Both halves come from one read, because both refusals that use them are about the
// same moment — a rewrite blocked by a running bot, a delete blocked by any bot —
// and two reads can disagree about it.
//
// It asks nothing about who wants to know. The caller has already established that
// the trading strategy is theirs; a bot pointing at it can only be theirs too.
func (strategyBotService *StrategyBotService) ReadReferencesTo(
	executionContext context.Context, tradingStrategyID uint,
) (dto.TradingStrategyReferencesDto, error) {
	bots, findError := strategyBotService.strategyBotRepository.FindAllByTradingStrategy(
		executionContext, tradingStrategyID)
	if findError != nil {
		return dto.TradingStrategyReferencesDto{}, findError
	}

	runningBotNames := make([]string, 0, len(bots))
	for _, bot := range bots {
		if vo.StrategyBotRunStateVo(bot.RunState) == vo.StrategyBotRunning {
			runningBotNames = append(runningBotNames, bot.Name)
		}
	}

	return dto.TradingStrategyReferencesDto{
		TotalCount:      len(bots),
		RunningBotNames: runningBotNames,
	}, nil
}

// StartStrategyBot puts the viewer's own bot to work, due immediately.
//
// Whether they can be spoken to is answered by the caller, because the delivery
// setting is not this service's to read. Starting an already running bot changes
// nothing and is not a failure — including not clearing what it has already sent,
// which a second press must not turn into a repeat message.
// The second return value says whether **this call** was the one that changed it.
// Pressing the button twice must not announce twice: the second press asks for a
// state the bot is already in, and a message saying so would be a message about
// nothing.
func (strategyBotService *StrategyBotService) StartStrategyBot(
	executionContext context.Context, viewerID uint, id uint, hasDeliverySetting bool,
) (dto.StrategyBotDto, bool, error) {
	storedBot, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, false, findError
	}

	runStateDomain := domains.NewStrategyBotRunStateDomain(storedBot)
	if runStateDomain.IsRunning() {
		return storedBot.ToDto(), false, nil
	}

	runningBotCount, countError := strategyBotService.strategyBotRepository.CountRunningByOwner(
		executionContext, storedBot.OwnerID)
	if countError != nil {
		return dto.StrategyBotDto{}, false, countError
	}

	if startableError := runStateDomain.RequireStartable(
		hasDeliverySetting, runningBotCount); startableError != nil {
		return dto.StrategyBotDto{}, false, startableError
	}

	startedBot := runStateDomain.Start(strategyBotService.clockProxy.Now())
	if updateError := strategyBotService.strategyBotRepository.UpdateRunState(
		executionContext, startedBot); updateError != nil {
		return dto.StrategyBotDto{}, false, updateError
	}

	return startedBot.ToDto(), true, nil
}

// StopStrategyBot takes the viewer's own bot off duty. Stopping one that is already
// stopped is not a failure: the state they asked for is the state it is in.
// The second return value says whether this call was the one that stopped it, for
// the same reason as starting: a second press asks for a state it is already in.
func (strategyBotService *StrategyBotService) StopStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, bool, error) {
	storedBot, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, false, findError
	}

	runStateDomain := domains.NewStrategyBotRunStateDomain(storedBot)
	wasRunning := runStateDomain.IsRunning()

	stoppedBot := runStateDomain.Stop()
	if updateError := strategyBotService.strategyBotRepository.UpdateRunState(
		executionContext, stoppedBot); updateError != nil {
		return dto.StrategyBotDto{}, false, updateError
	}

	return stoppedBot.ToDto(), wasRunning, nil
}

// FindDueStrategyBots returns running bots whose next round has come, oldest first
// and at most this many.
//
// It takes no viewer, and that is not an oversight. Nobody asked for these — the
// clock did — and there is no person whose permissions could be checked.
func (strategyBotService *StrategyBotService) FindDueStrategyBots(
	executionContext context.Context, limit int,
) ([]dto.StrategyBotDto, error) {
	bots, findError := strategyBotService.strategyBotRepository.FindDue(
		executionContext, strategyBotService.clockProxy.Now(), limit)
	if findError != nil {
		return nil, findError
	}

	botDtos := make([]dto.StrategyBotDto, 0, len(bots))
	for _, bot := range bots {
		botDtos = append(botDtos, bot.ToDto())
	}

	return botDtos, nil
}

// DecideRound reads what each source said this round through both conditions, and
// says what follows: the conclusion, whether it is worth sending, and whether the
// two conditions contradicted each other.
//
// It reads nothing and writes nothing. That is what lets the same rules be replayed
// over history one round at a time, should a backtest of a whole bot ever be asked
// for, without a line of this changing.
func (strategyBotService *StrategyBotService) DecideRound(
	botDto dto.StrategyBotDto, tradingStrategyDto dto.TradingStrategyDto,
	signalsByLabel map[string]vo.SignalVo,
) (dto.StrategyBotRoundDecisionDto, error) {
	declaredLabels := make([]string, 0, len(tradingStrategyDto.SignalSources))
	for _, signalSource := range tradingStrategyDto.SignalSources {
		declaredLabels = append(declaredLabels, signalSource.Label)
	}

	buyCondition, buyError := domains.NewTradingStrategyConditionDomain(
		tradingStrategyDto.BuyCondition, declaredLabels)
	if buyError != nil {
		return dto.StrategyBotRoundDecisionDto{}, buyError
	}

	sellCondition, sellError := domains.NewTradingStrategyConditionDomain(
		tradingStrategyDto.SellCondition, declaredLabels)
	if sellError != nil {
		return dto.StrategyBotRoundDecisionDto{}, sellError
	}

	verdictDomain := domains.NewStrategyBotVerdictDomain(
		buyCondition.Holds(signalsByLabel),
		sellCondition.Holds(signalsByLabel),
		botDto.LastSentSignal)

	return dto.StrategyBotRoundDecisionDto{
		Verdict:     string(verdictDomain.Verdict()),
		ShouldSend:  verdictDomain.ShouldSend(),
		Conflicting: verdictDomain.IsConflicting(),
	}, nil
}

// RequireCurrentMarket refuses a round whose market has stopped arriving, judged by the
// newest one-minute candle the round read for its reference price — which only a
// contract bot is asked, see StrategyBotMarketDomain.RequireCurrentMarket. The refusal
// is one the failure model reads as a skipped round.
func (strategyBotService *StrategyBotService) RequireCurrentMarket(
	botDto dto.StrategyBotDto, newestCandleOpenTime time.Time, hasNewestCandle bool,
) error {
	return domains.NewStrategyBotMarketDomain(botDto.MarketDataKind, "").RequireCurrentMarket(
		newestCandleOpenTime, hasNewestCandle, strategyBotService.clockProxy.Now())
}

// ReadRoundFailure says what a failure that happened during a round means: stop this
// bot and say why, or wait for the next one.
//
// It is here rather than at the call site because it is a domain question, and
// because it must have exactly one answer — a second place deciding it is a second
// list of which failures are hopeless, and the two will disagree the day a new one
// appears.
func (strategyBotService *StrategyBotService) ReadRoundFailure(
	roundError error,
) dto.StrategyBotRoundOutcomeDto {
	return domains.NewStrategyBotRoundFailureDomain(roundError).ToOutcomeDto()
}

// ReadDeliveryFailure says the same about a failure Telegram reported.
func (strategyBotService *StrategyBotService) ReadDeliveryFailure(
	failureReason string,
) dto.StrategyBotRoundOutcomeDto {
	return domains.NewStrategyBotDeliveryFailureDomain(
		vo.DeliveryFailureReasonVo(failureReason)).ToOutcomeDto()
}

// PlanRoundPosition is this round with what it suggests putting down worked out.
//
// It happens here, once, because the figures have two readers — the message its owner
// reads and the history they read it back in. Working them out in each would be two
// answers, and they would part company the moment somebody edited a setting between
// the two reads.
//
// Settings it cannot read leave the round exactly as it arrived. That is unreachable
// through the save gate, which refuses such settings before they are ever stored, and
// it is written down because the alternative to a rule is an accident: a bot whose
// figures cannot be read says nothing about them rather than saying something wrong.
//
// A contract bot's suggestion is worked out by the venue's rules, so for one — and only
// when there is something to suggest — the contract's trading rules and latest funding
// settlement are read first. A read that fails is taken as that half not being known:
// the suggestion then says what it could not account for, rather than the round going
// without one.
func (strategyBotService *StrategyBotService) PlanRoundPosition(
	executionContext context.Context, round dto.StrategyBotRoundDto,
) dto.StrategyBotRoundDto {
	positionPlan, positionPlanError := domains.NewPositionPlanDomain(round.PositionPlanSettings)
	if positionPlanError != nil {
		return round
	}

	// What this round's conclusion asks the account to hold is the whole of what a
	// plan needs: whether there is anything to suggest opening at all, and which way.
	// Which account that is — and on a contract one, which trading mode — decides it.
	market := domains.NewStrategyBotMarketDomain(round.MarketDataKind, round.ContractTradingMode)
	target := market.TargetFor(domains.NewSignalDomainOf(vo.SignalVo(round.Verdict)))

	if !market.IsContract() || !positionPlan.NeedsVenue(target, round.HasReference) {
		round.PositionPlan, round.HasPositionPlan = positionPlan.PlanFor(
			target, round.ReferencePrice, round.HasReference)

		return round
	}

	contractTradingSymbol, isRegistered, findSymbolError := strategyBotService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, round.Symbol)
	maintenanceMarginTiers, findTiersError := strategyBotService.
		contractMaintenanceMarginTierRepository.FindBySymbol(executionContext, round.Symbol)
	if findTiersError != nil {
		maintenanceMarginTiers = nil
	}
	latestSettlement, hasSettlement, findSettlementError := strategyBotService.
		contractFundingRateSettlementRepository.FindLatest(executionContext, round.Symbol)

	round.PositionPlan, round.HasPositionPlan = positionPlan.PlanOnContractVenue(
		target, round.ReferencePrice, round.ReferenceTime,
		domains.NewContractStrategyBotVenueDomain(
			contractTradingSymbol, isRegistered && findSymbolError == nil,
			maintenanceMarginTiers,
			latestSettlement, hasSettlement && findSettlementError == nil))

	return round
}

// WriteRoundMessage is this round as the message its owner reads.
func (strategyBotService *StrategyBotService) WriteRoundMessage(
	round dto.StrategyBotRoundDto,
) string {
	return domains.NewStrategyBotMessageDomain(round).Text()
}

// RecordRound books in whatever one round came to, and moves the bot on.
//
// One method for all three ways a round ends, rather than one each. A round has
// several exits, and with a method per exit the one that forgets to call its own
// leaves a bot due forever — running flat out against the database and Telegram,
// and looking from the outside exactly like a bot that is working.
// It hands back the bot as it now stands, and whether this round's outcome was
// applied at all — a round that arrives to find the bot has moved on writes nothing,
// and whoever called must not then announce something that did not happen.
func (strategyBotService *StrategyBotService) RecordRound(
	executionContext context.Context, id uint, dueAt time.Time,
	outcomeDto dto.StrategyBotRoundOutcomeDto,
) (dto.StrategyBotDto, bool, error) {
	outcome := domains.NewStrategyBotRoundOutcomeDomainOf(outcomeDto)

	storedBot, findError := strategyBotService.strategyBotRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyBotDto{}, false, findError
	}

	// This bot has to still be waiting for *this* round. Between a round starting
	// and finishing, its owner may have stopped and started it again — and starting
	// rewrites the very columns this write is about to touch, so putting the round's
	// values back would quietly undo the restart: the bot would not run immediately
	// as a start promises, and the signal it was told to forget would come back and
	// suppress the first conclusion after it.
	//
	// The due time is the token. Nothing else moves it, so a different one means
	// something else has already spoken for this bot.
	if !storedBot.NextRunAt.UTC().Equal(dueAt.UTC()) {
		return storedBot.ToDto(), false, nil
	}

	ranAt := strategyBotService.clockProxy.Now()
	endedBot := outcome.ApplyTo(domains.NewStrategyBotRunStateDomain(storedBot), ranAt)

	if updateError := strategyBotService.strategyBotRepository.UpdateRunState(
		executionContext, endedBot); updateError != nil {
		return dto.StrategyBotDto{}, false, updateError
	}

	// The history is written after the bot, and its failure is reported. Writing it
	// first would leave a round remembered that never happened; not reporting it
	// would let a bot's history quietly stop growing while the bot carried on, and
	// somebody would open it next month to find it ends in August.
	if appendError := strategyBotService.strategyBotRunRecordRepository.Append(
		executionContext, dto.StrategyBotRunRecordWriteDto{
			StrategyBotID: id,
			RanAt:         ranAt,
			Result:        string(outcome.RecordedResult()),
			// Carried through from the outcome rather than worked out again here: the
			// figures belong to the round that sent them, and this bot's settings may
			// already have changed.
			PositionPlan:    outcomeDto.PositionPlan,
			HasPositionPlan: outcomeDto.HasPositionPlan,
		}); appendError != nil {
		return dto.StrategyBotDto{}, false, appendError
	}

	return endedBot.ToDto(), true, nil
}

// WriteStartedMessage and WriteStoppedMessage are the two things a bot says about
// itself, rather than about the market.
//
// They are here because writing them is a domain decision — which words, and whether
// a halt says why — and because the layer that sends them may not build a model to
// ask.
func (strategyBotService *StrategyBotService) WriteStartedMessage(
	botDto dto.StrategyBotDto,
) string {
	return domains.NewStrategyBotLifecycleMessageDomain(
		botDto.Name,
		domains.NewStrategyBotMarketDomain(botDto.MarketDataKind, "").SymbolLabel(botDto.Symbol),
		vo.StrategyBotHaltNone).StartedText()
}

func (strategyBotService *StrategyBotService) WriteStoppedMessage(
	botDto dto.StrategyBotDto,
) string {
	return domains.NewStrategyBotLifecycleMessageDomain(
		botDto.Name,
		domains.NewStrategyBotMarketDomain(botDto.MarketDataKind, "").SymbolLabel(botDto.Symbol),
		vo.StrategyBotHaltReasonVo(botDto.HaltReason)).StoppedText()
}

// ListRunRecords is what this bot has been doing: its remembered rounds, newest
// first.
//
// It serves owners only, like everything else about a bot. A stranger is owed the
// same sentence as a bot that is not there.
func (strategyBotService *StrategyBotService) ListRunRecords(
	executionContext context.Context, viewerID uint, id uint,
) ([]dto.StrategyBotRunRecordDto, error) {
	if _, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id); findError != nil {
		return nil, findError
	}

	runRecords, listError := strategyBotService.strategyBotRunRecordRepository.FindLatestByBot(
		executionContext, id)
	if listError != nil {
		return nil, listError
	}

	runRecordDtos := make([]dto.StrategyBotRunRecordDto, 0, len(runRecords))
	for _, runRecord := range runRecords {
		runRecordDtos = append(runRecordDtos, runRecord.ToDto())
	}

	return runRecordDtos, nil
}

// settle is every rule a bot being saved has to pass, creating or rewriting: its own,
// following rules of the same kind of market, and — for a contract bot — watching a
// contract the system follows, with no more leverage than that contract allows.
//
// Both public writers need all of it in this order, and a rule answered in one and
// forgotten in the other is a bot that can be rewritten into something it could never
// have been created as.
func (strategyBotService *StrategyBotService) settle(
	executionContext context.Context, writeDto dto.StrategyBotWriteDto,
	followedTradingStrategy dto.TradingStrategyDto,
) (domains.StrategyBotDomain, error) {
	strategyBotDomain, validationError := domains.NewStrategyBotDomain(writeDto)
	if validationError != nil {
		return domains.StrategyBotDomain{}, validationError
	}

	if followError := strategyBotDomain.RequireFollowing(
		followedTradingStrategy.MarketDataKind); followError != nil {
		return domains.StrategyBotDomain{}, followError
	}

	if !strategyBotDomain.WatchesContracts() {
		return strategyBotDomain, nil
	}

	contractTradingSymbol, isRegistered, findSymbolError := strategyBotService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, strategyBotDomain.Symbol())
	if findSymbolError != nil {
		return domains.StrategyBotDomain{}, findSymbolError
	}

	maintenanceMarginTiers, findTiersError := strategyBotService.
		contractMaintenanceMarginTierRepository.FindBySymbol(executionContext, strategyBotDomain.Symbol())
	if findTiersError != nil {
		return domains.StrategyBotDomain{}, findTiersError
	}

	if admitError := domains.NewContractStrategyBotMarketDomain(
		strategyBotDomain.Symbol(), contractTradingSymbol, isRegistered, maintenanceMarginTiers,
	).Admit(strategyBotDomain.Leverage()); admitError != nil {
		return domains.StrategyBotDomain{}, admitError
	}

	return strategyBotDomain, nil
}

// findOwnedBot is the two steps in front of everything a person does to a bot: find
// it, then ask whether it is theirs. Every public method above needs both, and a
// stranger is owed the same sentence as a bot that is not there — told apart, anybody
// could walk the identifiers and learn which bots exist.
func (strategyBotService *StrategyBotService) findOwnedBot(
	executionContext context.Context, viewerID uint, id uint,
) (entities.StrategyBot, error) {
	bot, findError := strategyBotService.strategyBotRepository.FindOne(executionContext, id)
	if findError != nil {
		return entities.StrategyBot{}, findError
	}

	if bot.OwnerID != viewerID {
		return entities.StrategyBot{}, domains.StrategyBotNotFound(id)
	}

	return bot, nil
}
