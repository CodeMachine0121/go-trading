package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// unknownChatDescriptions are the phrases Telegram uses when the chat is the
// problem rather than the token.
//
// Matching on prose is unpleasant, and it is done here because Telegram gives no
// other handle: a bad token and an unknown chat both come back as a refusal, and
// only the sentence tells them apart. It is confined to this file, and everything
// it cannot recognise is reported as unreachable rather than guessed at — an
// unrecognised phrase makes somebody retry, whereas a wrong guess makes them
// replace a token that was never wrong.
var unknownChatDescriptions = []string{
	"chat not found",
	"chat_id is empty",
	"group chat was upgraded",
	"bot was blocked by the user",
	"bot was kicked",
	"user is deactivated",
	"peer_id_invalid",
}

// TelegramMessageDeliveryProxy sends messages through Telegram.
//
// Nothing it logs or returns ever contains the request address. That is not
// squeamishness: Telegram puts the bot token in the path, so an address written to
// a log file is a token written to a log file — and log files are the likeliest way
// a secret escapes, far ahead of anybody reading the database.
type TelegramMessageDeliveryProxy struct {
	apiBaseUrl string
	httpClient *http.Client
}

func NewTelegramMessageDeliveryProxy(
	apiBaseUrl string, httpClient *http.Client,
) *TelegramMessageDeliveryProxy {
	return &TelegramMessageDeliveryProxy{
		apiBaseUrl: strings.TrimRight(apiBaseUrl, "/"),
		httpClient: httpClient,
	}
}

// Deliver sends one message as one bot into one chat.
//
// Every outcome Telegram can produce lands in one of four reasons, including the
// ones nothing here recognises. There is no fifth "something else" for a caller to
// puzzle over, because the caller's job is to repeat the reason to a person, and a
// person cannot act on "something else".
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) Deliver(
	executionContext context.Context,
	credential vo.MessageDeliveryCredentialVo,
	message string,
) (vo.DeliveryFailureReasonVo, error) {
	body, encodeError := json.Marshal(telegramSendMessageRequest{
		ChatID: credential.ChatID,
		Text:   message,
	})
	if encodeError != nil {
		return vo.DeliveryFailureUnreachable, fmt.Errorf("build message: %w", encodeError)
	}

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodPost,
		telegramMessageDeliveryProxy.sendMessageAddress(credential.BotToken),
		bytes.NewReader(body))
	if buildError != nil {
		// The address is deliberately absent from this message. It carries the
		// token, and this error is on its way to somewhere it will be written down.
		return vo.DeliveryFailureUnreachable, errors.New("build message request failed")
	}
	request.Header.Set("Content-Type", "application/json")

	response, requestError := telegramMessageDeliveryProxy.httpClient.Do(request)
	if requestError != nil {
		// A request cut short because time ran out is reported as time running
		// out, not as an unreachable service. They lead somebody to wait
		// different lengths before deciding something is wrong.
		if errors.Is(requestError, context.DeadlineExceeded) {
			return vo.DeliveryFailureTimedOut, nil
		}

		return vo.DeliveryFailureUnreachable, nil
	}
	defer func() { _ = response.Body.Close() }()

	return telegramMessageDeliveryProxy.reasonFrom(response), nil
}

// reasonFrom reads Telegram's answer and says which of the four things happened.
//
// An answer that cannot be decoded at all is unreachable rather than an error: the
// person asked whether the route works, and "it answered something I cannot read"
// is an answer to that question, not a fault in the asking.
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) reasonFrom(
	response *http.Response,
) vo.DeliveryFailureReasonVo {
	sendMessageResponse := telegramSendMessageResponse{}
	decodeError := json.NewDecoder(response.Body).Decode(&sendMessageResponse)

	if decodeError == nil && sendMessageResponse.Ok {
		return vo.DeliveryFailureNone
	}
	if response.StatusCode == http.StatusOK && decodeError == nil {
		// A 200 that says ok:false is Telegram disagreeing with itself. Treated as
		// unreachable so that nobody is told to change something.
		return vo.DeliveryFailureUnreachable
	}

	// The token is what a 401 rejects, and Telegram also answers 404 for a token
	// it does not recognise at all — the path it was addressed with does not
	// exist, because the token is in the path.
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNotFound {
		return vo.DeliveryFailureCredentialRejected
	}

	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusForbidden {
		if describesUnknownChat(sendMessageResponse.Description) {
			return vo.DeliveryFailureDestinationNotFound
		}

		// A refusal whose wording is unfamiliar is not guessed at. Sending
		// somebody to regenerate a working token costs more than telling them to
		// try again.
		return vo.DeliveryFailureUnreachable
	}

	return vo.DeliveryFailureUnreachable
}

// sendMessageAddress is where Telegram is asked to send a message. The token sits in
// the path, which is why this value never appears in a log line or an error.
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) sendMessageAddress(
	botToken string,
) string {
	return telegramMessageDeliveryProxy.apiBaseUrl + "/bot" + botToken + "/sendMessage"
}

// describesUnknownChat says whether Telegram's sentence blames the chat.
func describesUnknownChat(description string) bool {
	loweredDescription := strings.ToLower(description)
	for _, phrase := range unknownChatDescriptions {
		if strings.Contains(loweredDescription, phrase) {
			return true
		}
	}

	return false
}
