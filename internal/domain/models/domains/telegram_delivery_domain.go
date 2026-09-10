package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// botTokenTailLength is how much of a bot token is ever shown again.
//
// Four characters is enough to answer the only question anybody asks of a stored
// token — "is that the one I pasted, or the old one?" — and nowhere near enough to
// reconstruct it. It is a constant rather than a setting because it is a decision
// about safety, and a deployment that could raise it could turn the mask off by
// accident.
const botTokenTailLength = 4

// TelegramDeliveryDomain holds one attempt to say where this system should speak to
// somebody, and guarantees the rules about it passed. An instance only exists when
// they did.
//
// Both halves are trimmed, unlike a password. The reason they differ is what the
// blanks mean: in a password they are characters somebody chose, whereas a token or
// a chat identifier pasted out of another window drags its surroundings along, and
// nobody ever meant those to be part of it.
type TelegramDeliveryDomain struct {
	botToken string
	chatID   string
}

// NewTelegramDeliveryDomain judges the bot token first and the chat second, so that
// somebody who left both blank is told about the harder one to get right first.
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

// BotToken is the token as given, for the one use it has on the way in: being
// sealed. Nothing reads it back out of storage through this model.
func (telegramDeliveryDomain TelegramDeliveryDomain) BotToken() string {
	return telegramDeliveryDomain.botToken
}

// ToEntity is this setting as it is stored, given the sealed form of its token.
//
// The sealed form arrives as an argument rather than being worked out here, because
// working it out is cryptography and the domain is not allowed to know any. What
// the domain does know is that the sealed form is the only form the token may be
// stored in — which is why this is the one way to get a row, and why the row it
// builds has nowhere to put the token itself.
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

// botTokenTail is the part of the token that may be shown again.
//
// A token shorter than the tail shows nothing at all rather than showing itself. It
// is not a case worth a warning — a real token is far longer — but it is exactly the
// case where "show the last four" quietly becomes "show all of it", and a rule that
// leaks on its own edge is not a rule.
func (telegramDeliveryDomain TelegramDeliveryDomain) botTokenTail() string {
	if len(telegramDeliveryDomain.botToken) <= botTokenTailLength {
		return ""
	}

	return telegramDeliveryDomain.botToken[len(telegramDeliveryDomain.botToken)-botTokenTailLength:]
}
