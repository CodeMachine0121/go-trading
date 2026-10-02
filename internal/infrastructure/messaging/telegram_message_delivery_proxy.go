package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TelegramMessageDeliveryProxy never logs or returns the request address, because Telegram puts the bot token in the path.
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

// Deliver maps every Telegram outcome, including unrecognised ones, to one of four failure reasons.
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) Deliver(
	executionContext context.Context,
	credential vo.MessageDeliveryCredentialVo,
	message string,
) (vo.DeliveryResultVo, error) {
	body, encodeError := json.Marshal(telegramSendMessageRequest{
		ChatID: credential.ChatID,
		Text:   message,
	})
	if encodeError != nil {
		return vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable}, fmt.Errorf("build message: %w", encodeError)
	}

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodPost,
		telegramMessageDeliveryProxy.sendMessageAddress(credential.BotToken),
		bytes.NewReader(body))
	if buildError != nil {
		// The address is omitted because it carries the token.
		return vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable}, errors.New("build message request failed")
	}
	request.Header.Set("Content-Type", "application/json")

	response, requestError := telegramMessageDeliveryProxy.httpClient.Do(request)
	if requestError != nil {
		// A timeout is reported distinctly from an unreachable service.
		if errors.Is(requestError, context.DeadlineExceeded) {
			return vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureTimedOut}, nil
		}

		return vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable}, nil
	}
	defer func() { _ = response.Body.Close() }()

	sendMessageResponse := telegramSendMessageResponse{}
	decodeError := json.NewDecoder(response.Body).Decode(&sendMessageResponse)

	// Being told to slow down is a temporary refusal, so it reads as unreachable and carries the wait Telegram asked for.
	if response.StatusCode == http.StatusTooManyRequests {
		return vo.DeliveryResultVo{
			FailureReason: vo.DeliveryFailureUnreachable,
			RetryAfter:    time.Duration(sendMessageResponse.Parameters.RetryAfter) * time.Second,
		}, nil
	}

	return vo.DeliveryResultVo{
		FailureReason: telegramMessageDeliveryProxy.reasonFrom(response, sendMessageResponse, decodeError),
	}, nil
}

// reasonFrom treats an undecodable answer as unreachable rather than an error.
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) reasonFrom(
	response *http.Response, sendMessageResponse telegramSendMessageResponse, decodeError error,
) vo.DeliveryFailureReasonVo {
	if decodeError == nil && sendMessageResponse.Ok {
		return vo.DeliveryFailureNone
	}
	if response.StatusCode == http.StatusOK && decodeError == nil {
		// A 200 with ok:false is treated as unreachable so nobody is told to change anything.
		return vo.DeliveryFailureUnreachable
	}

	// Telegram answers 404 for an unknown token, because the token is part of the path.
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNotFound {
		return vo.DeliveryFailureCredentialRejected
	}

	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusForbidden {
		if sendMessageResponse.BlamesTheChat() {
			return vo.DeliveryFailureDestinationNotFound
		}

		// Unfamiliar wording is not guessed at; telling someone to retry is cheaper than sending them to regenerate a working token.
		return vo.DeliveryFailureUnreachable
	}

	return vo.DeliveryFailureUnreachable
}

// sendMessageAddress contains the token in its path, so it must never appear in logs or errors.
func (telegramMessageDeliveryProxy *TelegramMessageDeliveryProxy) sendMessageAddress(
	botToken string,
) string {
	return telegramMessageDeliveryProxy.apiBaseUrl + "/bot" + botToken + "/sendMessage"
}
