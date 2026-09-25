package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type TelegramDeliveryApplication struct {
	telegramDeliveryService *service.TelegramDeliveryService
}

func NewTelegramDeliveryApplication(
	telegramDeliveryService *service.TelegramDeliveryService,
) *TelegramDeliveryApplication {
	return &TelegramDeliveryApplication{telegramDeliveryService: telegramDeliveryService}
}

func (telegramDeliveryApplication *TelegramDeliveryApplication) GetDeliverySetting(
	executionContext context.Context, userID uint,
) (dto.TelegramDeliveryDto, error) {
	return telegramDeliveryApplication.telegramDeliveryService.GetDeliverySetting(
		executionContext, userID)
}

func (telegramDeliveryApplication *TelegramDeliveryApplication) SaveDeliverySetting(
	executionContext context.Context, userID uint, writeDto dto.TelegramDeliveryWriteDto,
) (dto.TelegramDeliveryDto, error) {
	return telegramDeliveryApplication.telegramDeliveryService.SaveDeliverySetting(
		executionContext, userID, writeDto)
}

func (telegramDeliveryApplication *TelegramDeliveryApplication) RemoveDeliverySetting(
	executionContext context.Context, userID uint,
) error {
	return telegramDeliveryApplication.telegramDeliveryService.RemoveDeliverySetting(
		executionContext, userID)
}

func (telegramDeliveryApplication *TelegramDeliveryApplication) SendTestMessage(
	executionContext context.Context, userID uint, testMessageDto dto.TestMessageDto,
) (dto.TestMessageResultDto, error) {
	return telegramDeliveryApplication.telegramDeliveryService.SendTestMessage(
		executionContext, userID, testMessageDto)
}
