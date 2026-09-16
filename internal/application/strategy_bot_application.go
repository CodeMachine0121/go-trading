package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyBotApplication orchestrates everything a person does to a strategy bot.
//
// It joins three domain services, which is this layer's job and not theirs. A bot
// names strategies, so saving one has to ask the strategy rules whether those may be
// seen and what knobs they declare; starting one has to ask the delivery setting
// whether its owner can be spoken to at all. Neither question belongs to the bots.
type StrategyBotApplication struct {
	strategyBotService      *service.StrategyBotService
	strategyService         *service.StrategyService
	telegramDeliveryService *service.TelegramDeliveryService
}

func NewStrategyBotApplication(
	strategyBotService *service.StrategyBotService,
	strategyService *service.StrategyService,
	telegramDeliveryService *service.TelegramDeliveryService,
) *StrategyBotApplication {
	return &StrategyBotApplication{
		strategyBotService:      strategyBotService,
		strategyService:         strategyService,
		telegramDeliveryService: telegramDeliveryService,
	}
}

// CreateStrategyBot saves a new bot for whoever is signed in.
func (strategyBotApplication *StrategyBotApplication) CreateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	writeDto.OwnerID = viewerID

	resolvedWriteDto, resolveError := strategyBotApplication.withResolvedStrategies(
		executionContext, viewerID, writeDto)
	if resolveError != nil {
		return dto.StrategyBotDto{}, resolveError
	}

	return strategyBotApplication.strategyBotService.CreateStrategyBot(
		executionContext, resolvedWriteDto)
}

// UpdateStrategyBot rewrites one of this person's bots.
func (strategyBotApplication *StrategyBotApplication) UpdateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	resolvedWriteDto, resolveError := strategyBotApplication.withResolvedStrategies(
		executionContext, viewerID, writeDto)
	if resolveError != nil {
		return dto.StrategyBotDto{}, resolveError
	}

	return strategyBotApplication.strategyBotService.UpdateStrategyBot(
		executionContext, viewerID, resolvedWriteDto)
}

// ListStrategyBots returns this person's bots.
func (strategyBotApplication *StrategyBotApplication) ListStrategyBots(
	executionContext context.Context, viewerID uint,
) ([]dto.StrategyBotDto, error) {
	return strategyBotApplication.strategyBotService.ListStrategyBots(executionContext, viewerID)
}

// GetStrategyBot returns one of this person's bots.
func (strategyBotApplication *StrategyBotApplication) GetStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	return strategyBotApplication.strategyBotService.GetStrategyBot(executionContext, viewerID, id)
}

// DeleteStrategyBot removes one of this person's bots.
func (strategyBotApplication *StrategyBotApplication) DeleteStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return strategyBotApplication.strategyBotService.DeleteStrategyBot(
		executionContext, viewerID, id)
}

// StartStrategyBot puts one of this person's bots to work.
//
// Whether they have somewhere to be spoken to is read here, because that fact
// belongs to the delivery setting and a domain service does not call another one.
func (strategyBotApplication *StrategyBotApplication) StartStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	deliverySetting, deliveryError := strategyBotApplication.telegramDeliveryService.GetDeliverySetting(
		executionContext, viewerID)
	if deliveryError != nil {
		return dto.StrategyBotDto{}, deliveryError
	}

	startedBot, justStarted, startError := strategyBotApplication.strategyBotService.StartStrategyBot(
		executionContext, viewerID, id, deliverySetting.Configured)
	if startError != nil {
		return dto.StrategyBotDto{}, startError
	}

	if justStarted {
		strategyBotApplication.announce(
			executionContext, viewerID,
			strategyBotApplication.strategyBotService.WriteStartedMessage(startedBot))
	}

	return startedBot, nil
}

// StopStrategyBot takes one of this person's bots off duty.
func (strategyBotApplication *StrategyBotApplication) StopStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	stoppedBot, justStopped, stopError := strategyBotApplication.strategyBotService.StopStrategyBot(
		executionContext, viewerID, id)
	if stopError != nil {
		return dto.StrategyBotDto{}, stopError
	}

	if justStopped {
		strategyBotApplication.announce(
			executionContext, viewerID,
			strategyBotApplication.strategyBotService.WriteStoppedMessage(stoppedBot))
	}

	return stoppedBot, nil
}

// ListRunRecords is what this bot has been doing.
func (strategyBotApplication *StrategyBotApplication) ListRunRecords(
	executionContext context.Context, viewerID uint, id uint,
) ([]dto.StrategyBotRunRecordDto, error) {
	return strategyBotApplication.strategyBotService.ListRunRecords(
		executionContext, viewerID, id)
}

// announce sends a bot's own news, and lets it fail.
//
// Pressing stop is not undone because Telegram was busy: the bot **is** stopped,
// the button did what it said, and reporting a failure would leave somebody pressing
// it again at a bot that is already off. The message is a courtesy on top of an
// action that has already happened.
func (strategyBotApplication *StrategyBotApplication) announce(
	executionContext context.Context, viewerID uint, message string,
) {
	_, _ = strategyBotApplication.telegramDeliveryService.SendMessage(
		executionContext, viewerID, message)
}

// withResolvedStrategies fills each source in with what only its strategy can say:
// the name to show, and the knobs it declares.
//
// Resolving is also the gate. Naming a strategy that is not this person's and not on
// the marketplace fails here with the same sentence as naming one that does not
// exist, which is what stops a bot's sources becoming a way to probe for strategies.
//
// Both creating and rewriting need every step of this, which is what earns it a
// name of its own.
func (strategyBotApplication *StrategyBotApplication) withResolvedStrategies(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotWriteDto, error) {
	resolvedSources := make(
		[]dto.StrategyBotSignalSourceWriteDto, 0, len(writeDto.SignalSources))

	for _, signalSource := range writeDto.SignalSources {
		runnableStrategy, resolveError := strategyBotApplication.strategyService.ResolveRunnableStrategy(
			executionContext, viewerID, signalSource.StrategyID)
		if resolveError != nil {
			return dto.StrategyBotWriteDto{}, resolveError
		}

		// Only the declared knobs are taken. The script is deliberately left
		// behind: what a bot stores about a source is which strategy it names, so
		// that a strategy adopted from the marketplace is run without ever being
		// copied somewhere its adopter could read it.
		signalSource.DeclaredParameters = runnableStrategy.Parameters
		resolvedSources = append(resolvedSources, signalSource)
	}

	writeDto.SignalSources = resolvedSources

	return writeDto, nil
}
