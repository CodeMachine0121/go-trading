package messaging

import "strings"

// telegramSendMessageRequest keeps the chat ID as text because Telegram accepts both numeric IDs and @names.
type telegramSendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// telegramSendMessageResponse keeps Description because it is the only way to tell a rejected token from an unknown chat.
type telegramSendMessageResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// unknownChatDescriptions are the phrases that blame the chat rather than the token; Telegram offers no structured way to tell them apart.
var unknownChatDescriptions = []string{
	"chat not found",
	// The bot cannot message first until someone has started it, the likeliest first-send failure.
	"can't initiate conversation",
	"bot can't send messages to bots",
	"chat_id is empty",
	"group chat was upgraded",
	"bot was blocked by the user",
	"bot was kicked",
	"user is deactivated",
	"peer_id_invalid",
}

// BlamesTheChat returns false for unfamiliar wording, so the caller reports unreachable rather than guessing.
func (telegramSendMessageResponse telegramSendMessageResponse) BlamesTheChat() bool {
	loweredDescription := strings.ToLower(telegramSendMessageResponse.Description)
	for _, phrase := range unknownChatDescriptions {
		if strings.Contains(loweredDescription, phrase) {
			return true
		}
	}

	return false
}
