package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TelegramDeliveryService is the application layer's only entry point for delivery settings; reading never unseals the token, and only deliver does.
type TelegramDeliveryService struct {
	telegramDeliveryRepository domaininterface.ITelegramDeliveryRepository
	secretSealProxy            domaininterface.ISecretSealProxy
	messageDeliveryProxy       domaininterface.IMessageDeliveryProxy
}

func NewTelegramDeliveryService(
	telegramDeliveryRepository domaininterface.ITelegramDeliveryRepository,
	secretSealProxy domaininterface.ISecretSealProxy,
	messageDeliveryProxy domaininterface.IMessageDeliveryProxy,
) *TelegramDeliveryService {
	return &TelegramDeliveryService{
		telegramDeliveryRepository: telegramDeliveryRepository,
		secretSealProxy:            secretSealProxy,
		messageDeliveryProxy:       messageDeliveryProxy,
	}
}

// GetDeliverySetting returns the user's setting, or Configured=false when none exists, without ever unsealing the token.
func (telegramDeliveryService *TelegramDeliveryService) GetDeliverySetting(
	executionContext context.Context, userID uint,
) (dto.TelegramDeliveryDto, error) {
	delivery, findError := telegramDeliveryService.telegramDeliveryRepository.FindOneByUser(
		executionContext, userID)
	if errors.Is(findError, domains.ErrTelegramDeliveryNotConfigured) {
		return dto.TelegramDeliveryDto{Configured: false}, nil
	}
	if findError != nil {
		return dto.TelegramDeliveryDto{}, findError
	}

	return delivery.ToDto(), nil
}

// SaveDeliverySetting seals the token before upserting and refuses when sealing is unavailable rather than storing it in the clear.
func (telegramDeliveryService *TelegramDeliveryService) SaveDeliverySetting(
	executionContext context.Context, userID uint, writeDto dto.TelegramDeliveryWriteDto,
) (dto.TelegramDeliveryDto, error) {
	deliverySetting, validationError := domains.NewTelegramDeliveryDomain(writeDto)
	if validationError != nil {
		return dto.TelegramDeliveryDto{}, validationError
	}

	sealedBotToken, sealError := telegramDeliveryService.secretSealProxy.Seal(
		deliverySetting.BotToken())
	if sealError != nil {
		return dto.TelegramDeliveryDto{}, sealError
	}

	savedDelivery, saveError := telegramDeliveryService.telegramDeliveryRepository.Upsert(
		executionContext, deliverySetting.ToEntity(userID, sealedBotToken))
	if saveError != nil {
		return dto.TelegramDeliveryDto{}, saveError
	}

	return savedDelivery.ToDto(), nil
}

func (telegramDeliveryService *TelegramDeliveryService) RemoveDeliverySetting(
	executionContext context.Context, userID uint,
) error {
	return telegramDeliveryService.telegramDeliveryRepository.DeleteByUser(
		executionContext, userID)
}

// SendTestMessage sends a user-typed message using only the stored setting, so a token can never enter unsealed; a refusing destination is a result, not an error.
func (telegramDeliveryService *TelegramDeliveryService) SendTestMessage(
	executionContext context.Context, userID uint, testMessageDto dto.TestMessageDto,
) (dto.TestMessageResultDto, error) {
	testMessage, validationError := domains.NewTestMessageDomain(testMessageDto.Message)
	if validationError != nil {
		return dto.TestMessageResultDto{}, validationError
	}

	failureReason, deliverError := telegramDeliveryService.deliver(
		executionContext, userID, testMessage.Value())
	if deliverError != nil {
		return dto.TestMessageResultDto{}, deliverError
	}

	return failureReason.ToDto(), nil
}

// SendMessage sends a system-composed message without the typed-message length rules; refusals come back as a reason so callers can tell a rejected token from an unreachable Telegram.
func (telegramDeliveryService *TelegramDeliveryService) SendMessage(
	executionContext context.Context, userID uint, message string,
) (vo.DeliveryFailureReasonVo, error) {
	return telegramDeliveryService.deliver(executionContext, userID, message)
}

// deliver is the only path that unseals a bot token, kept as one method so unsealing has exactly one caller.
func (telegramDeliveryService *TelegramDeliveryService) deliver(
	executionContext context.Context, userID uint, message string,
) (vo.DeliveryFailureReasonVo, error) {
	delivery, findError := telegramDeliveryService.telegramDeliveryRepository.FindOneByUser(
		executionContext, userID)
	if findError != nil {
		return vo.DeliveryFailureNone, findError
	}

	botToken, unsealError := telegramDeliveryService.secretSealProxy.Unseal(
		delivery.SealedBotToken)
	if unsealError != nil {
		return vo.DeliveryFailureNone, unsealError
	}

	return telegramDeliveryService.messageDeliveryProxy.Deliver(
		executionContext,
		vo.MessageDeliveryCredentialVo{BotToken: botToken, ChatID: delivery.ChatID},
		message,
	)
}
