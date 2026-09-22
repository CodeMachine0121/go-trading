package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyBotApplication orchestrates everything a person does to a strategy bot.
//
// It joins three domain services, which is this layer's job and not theirs. A bot
// names a trading strategy, so saving one has to ask the trading strategy rules
// whether that one may be seen at all; starting one has to ask the delivery setting
// whether its owner can be spoken to. Neither question belongs to the bots.
type StrategyBotApplication struct {
	strategyBotService      *service.StrategyBotService
	tradingStrategyService  *service.TradingStrategyService
	telegramDeliveryService *service.TelegramDeliveryService
}

func NewStrategyBotApplication(
	strategyBotService *service.StrategyBotService,
	tradingStrategyService *service.TradingStrategyService,
	telegramDeliveryService *service.TelegramDeliveryService,
) *StrategyBotApplication {
	return &StrategyBotApplication{
		strategyBotService:      strategyBotService,
		tradingStrategyService:  tradingStrategyService,
		telegramDeliveryService: telegramDeliveryService,
	}
}

// CreateStrategyBot saves a new bot for whoever is signed in.
func (strategyBotApplication *StrategyBotApplication) CreateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	writeDto.OwnerID = viewerID

	if gateError := strategyBotApplication.refuseUnownedTradingStrategy(
		executionContext, viewerID, writeDto.TradingStrategyID); gateError != nil {
		return dto.StrategyBotDto{}, gateError
	}

	return strategyBotApplication.strategyBotService.CreateStrategyBot(executionContext, writeDto)
}

// UpdateStrategyBot rewrites one of this person's bots.
func (strategyBotApplication *StrategyBotApplication) UpdateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	if gateError := strategyBotApplication.refuseUnownedTradingStrategy(
		executionContext, viewerID, writeDto.TradingStrategyID); gateError != nil {
		return dto.StrategyBotDto{}, gateError
	}

	return strategyBotApplication.strategyBotService.UpdateStrategyBot(
		executionContext, viewerID, writeDto)
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

// refuseUnownedTradingStrategy refuses a bot that names a set of rules this person
// cannot see.
//
// Naming somebody else's fails here with the same sentence as naming one that does
// not exist, which is what stops the field becoming a way to probe for other
// people's trading strategies. Nothing else about the rules is checked — they were
// checked once, where they live.
//
// Naming nothing passes, and is not refused here — that a bot must name exactly one
// set of rules is a rule about bots, answered where bots are validated.
func (strategyBotApplication *StrategyBotApplication) refuseUnownedTradingStrategy(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) error {
	if tradingStrategyID == 0 {
		return nil
	}

	_, findError := strategyBotApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, tradingStrategyID)

	return findError
}
