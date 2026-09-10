package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TelegramDeliveryService is the application layer's only entry point for where this
// system speaks to somebody. Its four public use-case methods never call one
// another.
//
// Reading a setting and sending a message look like they should share a step, and
// deliberately do not: reading never opens a sealed token, and sending is the only
// thing that does. Sharing a step would give the one dangerous operation a second
// caller, which is exactly the thing this design spends effort avoiding.
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

// GetDeliverySetting says where this person has asked to be spoken to.
//
// Having never set one up comes back as a setting that says so, not as an error.
// It is the ordinary state of somebody who has not got round to it, and a caller
// told it was a failure would have to decide which failures are really nothing.
//
// Nothing here opens the sealed token, and there is no branch that could.
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

// SaveDeliverySetting stores where this person wants to be spoken to, replacing
// whatever they had before.
//
// The token is sealed before anything is written, and a system with nothing to seal
// it with refuses here rather than storing it in the open. That refusal is the
// feature: an unsealed token would work perfectly, and would go on working right up
// until somebody read the table.
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

// RemoveDeliverySetting stops this system being able to speak to this person.
func (telegramDeliveryService *TelegramDeliveryService) RemoveDeliverySetting(
	executionContext context.Context, userID uint,
) error {
	return telegramDeliveryService.telegramDeliveryRepository.DeleteByUser(
		executionContext, userID)
}

// SendTestMessage sends one message this person typed, to the place they said, as
// the bot they said.
//
// It reads the setting that is stored rather than accepting one alongside the
// message. Accepting one would open a second way for a whole token to enter the
// system — one that never passes through sealing — and the value of "a token can
// only go in" collapses the moment there are two doors.
//
// A destination that refuses is a result, not a failure. The whole point of this is
// to find out, and finding out that the chat identifier is wrong is the button
// working.
func (telegramDeliveryService *TelegramDeliveryService) SendTestMessage(
	executionContext context.Context, userID uint, testMessageDto dto.TestMessageDto,
) (dto.TestMessageResultDto, error) {
	testMessage, validationError := domains.NewTestMessageDomain(testMessageDto.Message)
	if validationError != nil {
		return dto.TestMessageResultDto{}, validationError
	}

	delivery, findError := telegramDeliveryService.telegramDeliveryRepository.FindOneByUser(
		executionContext, userID)
	if findError != nil {
		return dto.TestMessageResultDto{}, findError
	}

	botToken, unsealError := telegramDeliveryService.secretSealProxy.Unseal(
		delivery.SealedBotToken)
	if unsealError != nil {
		return dto.TestMessageResultDto{}, unsealError
	}

	failureReason, deliverError := telegramDeliveryService.messageDeliveryProxy.Deliver(
		executionContext,
		vo.MessageDeliveryCredentialVo{BotToken: botToken, ChatID: delivery.ChatID},
		testMessage.Value(),
	)
	if deliverError != nil {
		return dto.TestMessageResultDto{}, deliverError
	}

	return failureReason.ToDto(), nil
}
