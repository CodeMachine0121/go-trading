package messaging

import "strings"

// telegramSendMessageRequest is what Telegram is asked to send.
//
// The chat identifier is text rather than a number, because Telegram accepts both a
// numeric chat and an @name, and turning one into the other here would refuse half
// the identifiers people actually have.
type telegramSendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// telegramSendMessageResponse is the part of Telegram's answer worth keeping.
//
// Description is the sentence it writes for people, and it is the only way to tell
// a rejected token from an unknown chat: both arrive with the same status. Reading
// it is unpleasant and is confined to this file, where it is a wire detail rather
// than a rule.
type telegramSendMessageResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// unknownChatDescriptions are the phrases Telegram uses when the chat is the problem
// rather than the token.
//
// Matching on prose is unpleasant, and it is done because Telegram gives no other
// handle: a bad token and an unknown chat both come back as a refusal, and only the
// sentence tells them apart. It lives beside the field it reads, and nothing outside
// this file ever sees a description.
var unknownChatDescriptions = []string{
	"chat not found",
	// Nobody has ever messaged this bot, so Telegram will not let it speak first.
	// It is the likeliest thing to go wrong on the very first send, and it is the
	// chat's problem rather than the token's: the token is fine, and no amount of
	// retrying helps — somebody has to open Telegram and say hello.
	"can't initiate conversation",
	"bot can't send messages to bots",
	"chat_id is empty",
	"group chat was upgraded",
	"bot was blocked by the user",
	"bot was kicked",
	"user is deactivated",
	"peer_id_invalid",
}

// BlamesTheChat says whether this answer puts the fault on the chat rather than on
// the bot speaking.
//
// An answer whose wording is unfamiliar says no, and the caller reports it as
// unreachable rather than guessing. Sending somebody off to regenerate a working
// token costs more than telling them to try again in a moment.
func (telegramSendMessageResponse telegramSendMessageResponse) BlamesTheChat() bool {
	loweredDescription := strings.ToLower(telegramSendMessageResponse.Description)
	for _, phrase := range unknownChatDescriptions {
		if strings.Contains(loweredDescription, phrase) {
			return true
		}
	}

	return false
}
