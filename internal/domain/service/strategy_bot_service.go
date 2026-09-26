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

// It deliberately cannot reach scripts, K candles or Telegram (the application layer joins those); the contract readers are needed only to validate contract bots and plan contract positions.
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

// CreateStrategyBot saves a new, stopped bot; the followed trading strategy's ownership is checked by the caller, and here only its market kind must match.
func (strategyBotService *StrategyBotService) CreateStrategyBot(
	executionContext context.Context, writeDto dto.StrategyBotWriteDto,
	followedTradingStrategy dto.TradingStrategyDto,
) (dto.StrategyBotDto, error) {
	// Clearing the ID stops a create from overwriting an existing (possibly foreign) bot.
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

// ListStrategyBots returns the owner's bots, optionally narrowed to one market kind; an unknown kind is refused so it cannot read as "you have none".
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

func (strategyBotService *StrategyBotService) GetStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	bot, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id)
	if findError != nil {
		return dto.StrategyBotDto{}, findError
	}

	return bot.ToDto(), nil
}

// UpdateStrategyBot rewrites a stopped bot under the same rules as creation, keeping the market kind it was created with.
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

	// The owner always comes from storage, so a bot cannot change hands.
	writeDto.OwnerID = storedBot.OwnerID

	// Read through the kind domain so bots stored before the choice existed keep their original kind.
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

// DeleteStrategyBot removes the viewer's bot even while running, since a deleted bot is never picked up again.
func (strategyBotService *StrategyBotService) DeleteStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := strategyBotService.findOwnedBot(executionContext, viewerID, id); findError != nil {
		return findError
	}

	return strategyBotService.strategyBotRepository.Delete(executionContext, id)
}

// ReadReferencesTo counts the bots following a trading strategy and names the running ones, from one read so both refusals see the same moment.
// The caller has already established ownership of the strategy.
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

// ReadReferencesToStrategyScript names only the owner's running bots, since those are the ones the owner can stop; bots only ever run their owner's scripts.
func (strategyBotService *StrategyBotService) ReadReferencesToStrategyScript(
	executionContext context.Context, ownerID uint, strategyScriptID uint,
) (dto.StrategyScriptReferencesDto, error) {
	bots, findError := strategyBotService.strategyBotRepository.FindAllByStrategyScript(
		executionContext, strategyScriptID)
	if findError != nil {
		return dto.StrategyScriptReferencesDto{}, findError
	}

	runningBotNames := make([]string, 0, len(bots))
	for _, bot := range bots {
		if bot.OwnerID == ownerID && vo.StrategyBotRunStateVo(bot.RunState) == vo.StrategyBotRunning {
			runningBotNames = append(runningBotNames, bot.Name)
		}
	}

	return dto.StrategyScriptReferencesDto{
		TotalCount:      len(bots),
		RunningBotNames: runningBotNames,
	}, nil
}

// StartStrategyBot starts the bot, due immediately, and reports whether this call changed its state so a repeated press does not announce twice.
// The caller answers whether a delivery setting exists; starting a running bot is a no-op that keeps its sent-signal memory.
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

// StopStrategyBot stops the bot, reporting whether this call changed its state; stopping a stopped bot is not a failure.
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

// FindDueStrategyBots returns up to limit running bots whose next round has come, oldest first, with no viewer since the clock asked.
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

// DecideRound evaluates the buy and sell conditions against this round's signals, with no I/O, so it can be replayed over history.
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

// RequireCurrentMarket refuses a round whose market data has stopped arriving (checked only for contract bots); the failure model reads the refusal as a skipped round.
func (strategyBotService *StrategyBotService) RequireCurrentMarket(
	botDto dto.StrategyBotDto, newestCandleOpenTime time.Time, hasNewestCandle bool,
) error {
	return domains.NewStrategyBotMarketDomain(botDto.MarketDataKind, "").RequireCurrentMarket(
		newestCandleOpenTime, hasNewestCandle, strategyBotService.clockProxy.Now())
}

// ReadRoundFailure decides whether a round failure halts the bot or waits for the next round, kept in one place so there is one list of hopeless failures.
func (strategyBotService *StrategyBotService) ReadRoundFailure(
	roundError error,
) dto.StrategyBotRoundOutcomeDto {
	return domains.NewStrategyBotRoundFailureDomain(roundError).ToOutcomeDto()
}

// ReadDeliveryFailure does the same for a failure Telegram reported.
func (strategyBotService *StrategyBotService) ReadDeliveryFailure(
	failureReason string,
) dto.StrategyBotRoundOutcomeDto {
	return domains.NewStrategyBotDeliveryFailureDomain(
		vo.DeliveryFailureReasonVo(failureReason)).ToOutcomeDto()
}

// PlanRoundPosition computes the suggested position once so the message and the history show the same figures.
// Unreadable settings leave the round unchanged; contract venue reads happen only when there is something to suggest, and a failed read is treated as unknown.
func (strategyBotService *StrategyBotService) PlanRoundPosition(
	executionContext context.Context, round dto.StrategyBotRoundDto,
) dto.StrategyBotRoundDto {
	positionPlan, positionPlanError := domains.NewPositionPlanDomain(round.PositionPlanSettings)
	if positionPlanError != nil {
		return round
	}

	// The target holding depends on the account and, for contracts, the trading mode.
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

func (strategyBotService *StrategyBotService) WriteRoundMessage(
	round dto.StrategyBotRoundDto,
) string {
	return domains.NewStrategyBotMessageDomain(round).Text()
}

// RecordRound applies any round outcome through one method, so no exit can forget to reschedule the bot and leave it due forever.
// It reports whether the outcome was applied; a bot that moved on meanwhile is left untouched.
func (strategyBotService *StrategyBotService) RecordRound(
	executionContext context.Context, id uint, dueAt time.Time,
	outcomeDto dto.StrategyBotRoundOutcomeDto,
) (dto.StrategyBotDto, bool, error) {
	outcome := domains.NewStrategyBotRoundOutcomeDomainOf(outcomeDto)

	storedBot, findError := strategyBotService.strategyBotRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.StrategyBotDto{}, false, findError
	}

	// dueAt is the token: a stop-and-restart during the round moves NextRunAt, and writing the round back would silently undo the restart.
	if !storedBot.NextRunAt.UTC().Equal(dueAt.UTC()) {
		return storedBot.ToDto(), false, nil
	}

	ranAt := strategyBotService.clockProxy.Now()
	endedBot := outcome.ApplyTo(domains.NewStrategyBotRunStateDomain(storedBot), ranAt)

	if updateError := strategyBotService.strategyBotRepository.UpdateRunState(
		executionContext, endedBot); updateError != nil {
		return dto.StrategyBotDto{}, false, updateError
	}

	// History is written after the bot state so no phantom round is recorded, and its failure is reported so history cannot silently stop growing.
	if appendError := strategyBotService.strategyBotRunRecordRepository.Append(
		executionContext, dto.StrategyBotRunRecordWriteDto{
			StrategyBotID: id,
			RanAt:         ranAt,
			Result:        string(outcome.RecordedResult()),
			// Taken from the outcome because the bot's settings may have changed since the round was sent.
			PositionPlan:    outcomeDto.PositionPlan,
			HasPositionPlan: outcomeDto.HasPositionPlan,
		}); appendError != nil {
		return dto.StrategyBotDto{}, false, appendError
	}

	return endedBot.ToDto(), true, nil
}

// WriteStartedMessage and WriteStoppedMessage write the bot's lifecycle messages, a domain decision the sending layer cannot make itself.
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

// ListRunRecords returns the bot's remembered rounds, newest first, to its owner only.
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

// settle applies every save rule for both create and update, so a bot cannot be rewritten into something that could never have been created.
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

// findOwnedBot answers a stranger exactly as for a missing bot, so identifiers cannot be probed for existence.
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
