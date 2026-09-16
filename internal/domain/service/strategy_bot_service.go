package service

import (
	"context"
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
// It is given no way to reach a strategy, a K candle or Telegram, and that is the
// point. Whether a bot may name a strategy is the strategy rules' question; whether
// its owner can be spoken to is the delivery setting's; and joining those to this is
// the application layer's job. A dependency that is not here cannot be reached for
// by accident later.
type StrategyBotService struct {
	strategyBotRepository          domaininterface.IStrategyBotRepository
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository
	clockProxy                     domaininterface.IClockProxy
}

func NewStrategyBotService(
	strategyBotRepository domaininterface.IStrategyBotRepository,
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository,
	clockProxy domaininterface.IClockProxy,
) *StrategyBotService {
	return &StrategyBotService{
		strategyBotRepository:          strategyBotRepository,
		strategyBotRunRecordRepository: strategyBotRunRecordRepository,
		clockProxy:                     clockProxy,
	}
}

// CreateStrategyBot saves a new bot for its owner, stopped, and hands it back as
// stored. A bot that breaks a rule is refused before anything is written.
func (strategyBotService *StrategyBotService) CreateStrategyBot(
	executionContext context.Context, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	// An identifier arriving on a create would rewrite whichever bot it named,
	// including somebody else's. Clearing it makes creating unable to mean that.
	writeDto.ID = 0

	strategyBotDomain, validationError := domains.NewStrategyBotDomain(writeDto)
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

// ListStrategyBots returns this person's bots, by name.
//
// Having none is an empty list rather than a refusal: it is the ordinary state of
// somebody who has not built one yet.
func (strategyBotService *StrategyBotService) ListStrategyBots(
	executionContext context.Context, ownerID uint,
) ([]dto.StrategyBotDto, error) {
	bots, findError := strategyBotService.strategyBotRepository.FindAllByOwner(
		executionContext, ownerID)
	if findError != nil {
		return nil, findError
	}

	botDtos := make([]dto.StrategyBotDto, 0, len(bots))
	for _, bot := range bots {
		botDtos = append(botDtos, bot.ToDto())
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
// arrive as the same shape and are checked by the same model.
func (strategyBotService *StrategyBotService) UpdateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
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

	strategyBotDomain, validationError := domains.NewStrategyBotDomain(writeDto)
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
	botDto dto.StrategyBotDto, signalsByLabel map[string]vo.SignalVo,
) (dto.StrategyBotRoundDecisionDto, error) {
	declaredLabels := make([]string, 0, len(botDto.SignalSources))
	for _, signalSource := range botDto.SignalSources {
		declaredLabels = append(declaredLabels, signalSource.Label)
	}

	buyCondition, buyError := domains.NewStrategyBotConditionDomain(
		botDto.BuyCondition, declaredLabels)
	if buyError != nil {
		return dto.StrategyBotRoundDecisionDto{}, buyError
	}

	sellCondition, sellError := domains.NewStrategyBotConditionDomain(
		botDto.SellCondition, declaredLabels)
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
		executionContext, id, ranAt, string(outcome.RecordedResult())); appendError != nil {
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
		botDto.Name, botDto.Symbol, vo.StrategyBotHaltNone).StartedText()
}

func (strategyBotService *StrategyBotService) WriteStoppedMessage(
	botDto dto.StrategyBotDto,
) string {
	return domains.NewStrategyBotLifecycleMessageDomain(
		botDto.Name, botDto.Symbol,
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
