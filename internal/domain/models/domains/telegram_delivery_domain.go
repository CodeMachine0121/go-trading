package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// botTokenTailLength is how many trailing token characters may be shown again; a constant, not a setting, so the mask can't be disabled by configuration.
const botTokenTailLength = 4

// TelegramDeliveryDomain is a validated delivery setting; unlike passwords, both values are trimmed because pasted blanks are never intended.
type TelegramDeliveryDomain struct {
	botToken string
	chatID   string
}

// NewTelegramDeliveryDomain checks the bot token before the chat so the harder one is reported first.
func NewTelegramDeliveryDomain(
	telegramDeliveryWriteDto dto.TelegramDeliveryWriteDto,
) (TelegramDeliveryDomain, error) {
	botToken := strings.TrimSpace(telegramDeliveryWriteDto.BotToken)
	if botToken == "" {
		return TelegramDeliveryDomain{}, fmt.Errorf(
			"%w: 必須給一組機器人金鑰", ErrTelegramDeliveryValidation)
	}

	chatID := strings.TrimSpace(telegramDeliveryWriteDto.ChatID)
	if chatID == "" {
		return TelegramDeliveryDomain{}, fmt.Errorf(
			"%w: 必須給一個聊天室代號", ErrTelegramDeliveryValidation)
	}

	return TelegramDeliveryDomain{botToken: botToken, chatID: chatID}, nil
}

// BotToken is exposed only so it can be sealed on the way in.
func (telegramDeliveryDomain TelegramDeliveryDomain) BotToken() string {
	return telegramDeliveryDomain.botToken
}

// ToEntity takes the already-sealed token because sealing is crypto the domain must not know; the row has no field for the plain token.
func (telegramDeliveryDomain TelegramDeliveryDomain) ToEntity(
	userID uint, sealedBotToken string,
) entities.TelegramDelivery {
	return entities.TelegramDelivery{
		UserID:         userID,
		SealedBotToken: sealedBotToken,
		BotTokenTail:   telegramDeliveryDomain.botTokenTail(),
		ChatID:         telegramDeliveryDomain.chatID,
	}
}

// botTokenTail shows nothing for tokens no longer than the tail, so the mask can't leak the whole token.
func (telegramDeliveryDomain TelegramDeliveryDomain) botTokenTail() string {
	if len(telegramDeliveryDomain.botToken) <= botTokenTailLength {
		return ""
	}

	return telegramDeliveryDomain.botToken[len(telegramDeliveryDomain.botToken)-botTokenTailLength:]
}
