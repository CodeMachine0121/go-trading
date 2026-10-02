package application

import (
	"context"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// StrategyBotApplication joins the bot, trading strategy, delivery and trading key services, since a domain service does not call another.
type StrategyBotApplication struct {
	strategyBotService       *service.StrategyBotService
	tradingStrategyService   *service.TradingStrategyService
	telegramDeliveryService  *service.TelegramDeliveryService
	binanceTradingKeyService *service.BinanceTradingKeyService
	pendingMessageService    *service.PendingMessageService
}

func NewStrategyBotApplication(
	strategyBotService *service.StrategyBotService,
	tradingStrategyService *service.TradingStrategyService,
	telegramDeliveryService *service.TelegramDeliveryService,
	binanceTradingKeyService *service.BinanceTradingKeyService,
	pendingMessageService *service.PendingMessageService,
) *StrategyBotApplication {
	return &StrategyBotApplication{
		strategyBotService:       strategyBotService,
		tradingStrategyService:   tradingStrategyService,
		telegramDeliveryService:  telegramDeliveryService,
		binanceTradingKeyService: binanceTradingKeyService,
		pendingMessageService:    pendingMessageService,
	}
}

func (strategyBotApplication *StrategyBotApplication) CreateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	writeDto.OwnerID = viewerID

	followedTradingStrategy, readError := strategyBotApplication.readFollowedTradingStrategy(
		executionContext, viewerID, writeDto.TradingStrategyID)
	if readError != nil {
		return dto.StrategyBotDto{}, readError
	}

	return strategyBotApplication.strategyBotService.CreateStrategyBot(
		executionContext, writeDto, followedTradingStrategy)
}

func (strategyBotApplication *StrategyBotApplication) UpdateStrategyBot(
	executionContext context.Context, viewerID uint, writeDto dto.StrategyBotWriteDto,
) (dto.StrategyBotDto, error) {
	followedTradingStrategy, readError := strategyBotApplication.readFollowedTradingStrategy(
		executionContext, viewerID, writeDto.TradingStrategyID)
	if readError != nil {
		return dto.StrategyBotDto{}, readError
	}

	return strategyBotApplication.strategyBotService.UpdateStrategyBot(
		executionContext, viewerID, writeDto, followedTradingStrategy)
}

func (strategyBotApplication *StrategyBotApplication) ListStrategyBots(
	executionContext context.Context, viewerID uint, marketDataKind string,
) ([]dto.StrategyBotDto, error) {
	return strategyBotApplication.strategyBotService.ListStrategyBots(
		executionContext, viewerID, marketDataKind)
}

func (strategyBotApplication *StrategyBotApplication) GetStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	return strategyBotApplication.strategyBotService.GetStrategyBot(executionContext, viewerID, id)
}

func (strategyBotApplication *StrategyBotApplication) DeleteStrategyBot(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return strategyBotApplication.strategyBotService.DeleteStrategyBot(
		executionContext, viewerID, id)
}

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
			executionContext, startedBot,
			strategyBotApplication.strategyBotService.WriteStartedMessage(startedBot))
	}

	return startedBot, nil
}

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
			executionContext, stoppedBot,
			strategyBotApplication.strategyBotService.WriteStoppedMessage(stoppedBot))
	}

	return stoppedBot, nil
}

func (strategyBotApplication *StrategyBotApplication) EnableAutoOrder(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	binanceTradingKeyStatus, statusError := strategyBotApplication.binanceTradingKeyService.
		GetTradingKeyStatus(executionContext, viewerID)
	if statusError != nil {
		return dto.StrategyBotDto{}, statusError
	}

	return strategyBotApplication.strategyBotService.EnableAutoOrder(
		executionContext, viewerID, id, binanceTradingKeyStatus)
}

func (strategyBotApplication *StrategyBotApplication) DisableAutoOrder(
	executionContext context.Context, viewerID uint, id uint,
) (dto.StrategyBotDto, error) {
	return strategyBotApplication.strategyBotService.DisableAutoOrder(executionContext, viewerID, id)
}

func (strategyBotApplication *StrategyBotApplication) ListRunRecords(
	executionContext context.Context, viewerID uint, id uint,
) ([]dto.StrategyBotRunRecordDto, error) {
	return strategyBotApplication.strategyBotService.ListRunRecords(
		executionContext, viewerID, id)
}

// announce queues a bot notification and only logs a failure, since the action it reports has already happened.
func (strategyBotApplication *StrategyBotApplication) announce(
	executionContext context.Context, botDto dto.StrategyBotDto, message string,
) {
	if enqueueError := strategyBotApplication.pendingMessageService.EnqueueLifecycleMessage(
		executionContext, botDto.ID, botDto.OwnerID, message); enqueueError != nil {
		log.Printf("strategy bot %d: could not queue its notification: %v", botDto.ID, enqueueError)
	}
}

// readFollowedTradingStrategy fails for someone else's strategy with the same error as a missing one, so the field can't probe other users' strategies; naming nothing passes here.
func (strategyBotApplication *StrategyBotApplication) readFollowedTradingStrategy(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (dto.TradingStrategyDto, error) {
	if tradingStrategyID == 0 {
		return dto.TradingStrategyDto{}, nil
	}

	return strategyBotApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, tradingStrategyID)
}
