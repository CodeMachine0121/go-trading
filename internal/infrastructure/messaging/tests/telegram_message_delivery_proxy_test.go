package messaging_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aCredential is the pair every one of these sends with, so that a test about an
// outcome is not also a test about what was in the token.
var aCredential = vo.MessageDeliveryCredentialVo{
	BotToken: "123456:AAHqwertyuiop1234",
	ChatID:   "987654",
}

// telegramAnswering stands in for Telegram, answering with one status and one body.
func telegramAnswering(
	t *testing.T, statusCode int, body string,
) *messaging.TelegramMessageDeliveryProxy {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(statusCode)
			_, _ = writer.Write([]byte(body))
		}))
	t.Cleanup(server.Close)

	return messaging.NewTelegramMessageDeliveryProxy(server.URL, server.Client())
}

func TestTelegramMessageDeliveryProxyReportsAMessageThatWentThrough(t *testing.T) {
	proxy := telegramAnswering(t, http.StatusOK, `{"ok":true}`)

	reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

	require.NoError(t, err)
	assert.Equal(t, vo.DeliveryFailureNone, reason)
}

// The token travels in the path, which is why the address never appears in a log
// line or an error — and why this test checks it is where Telegram expects it.
func TestTelegramMessageDeliveryProxySendsAsTheBotIntoTheChat(t *testing.T) {
	sentPath := ""
	sentBody := ""
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			sentPath = request.URL.Path
			readBody, _ := io.ReadAll(request.Body)
			sentBody = string(readBody)
			_, _ = writer.Write([]byte(`{"ok":true}`))
		}))
	t.Cleanup(server.Close)

	proxy := messaging.NewTelegramMessageDeliveryProxy(server.URL+"/", server.Client())

	_, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

	require.NoError(t, err)
	assert.Equal(t, "/bot123456:AAHqwertyuiop1234/sendMessage", sentPath,
		"多給一條斜線的位址不該變成兩條")

	sentMessage := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(sentBody), &sentMessage))
	assert.Equal(t, "987654", sentMessage["chat_id"])
	assert.Equal(t, "哈囉", sentMessage["text"],
		"測試訊息照使用者打的字面送出，不做任何格式解讀")
}

func TestTelegramMessageDeliveryProxySortsEveryRefusalIntoOneOfTheFourReasons(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		body           string
		expectedReason vo.DeliveryFailureReasonVo
	}{
		{
			name:           "a token the destination will not accept",
			statusCode:     http.StatusUnauthorized,
			body:           `{"ok":false,"error_code":401,"description":"Unauthorized"}`,
			expectedReason: vo.DeliveryFailureCredentialRejected,
		},
		{
			// The token is in the path, so a token nothing recognises makes the
			// path itself not exist.
			name:           "a path that does not exist because the token does not",
			statusCode:     http.StatusNotFound,
			body:           `{"ok":false,"error_code":404,"description":"Not Found"}`,
			expectedReason: vo.DeliveryFailureCredentialRejected,
		},
		{
			name:           "a chat the destination does not know",
			statusCode:     http.StatusBadRequest,
			body:           `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`,
			expectedReason: vo.DeliveryFailureDestinationNotFound,
		},
		{
			name:           "a chat that blocked the bot",
			statusCode:     http.StatusForbidden,
			body:           `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`,
			expectedReason: vo.DeliveryFailureDestinationNotFound,
		},
		{
			// The likeliest first-run failure of all: a correct token, a correct
			// chat, and nobody has said hello to the bot yet. Reported as the
			// service being unreachable, somebody retries forever; reported as the
			// chat, they are pointed at the one thing that fixes it.
			name:           "a person who has never started the bot",
			statusCode:     http.StatusForbidden,
			body:           `{"ok":false,"error_code":403,"description":"Forbidden: bot can't initiate conversation with a user"}`,
			expectedReason: vo.DeliveryFailureDestinationNotFound,
		},
		{
			name:           "a chat with no identifier at all",
			statusCode:     http.StatusBadRequest,
			body:           `{"ok":false,"error_code":400,"description":"Bad Request: chat_id is empty"}`,
			expectedReason: vo.DeliveryFailureDestinationNotFound,
		},
		{
			// Not guessed at. Sending somebody off to regenerate a working token
			// costs more than telling them to try again in a moment.
			name:           "a refusal worded in a way nothing here recognises",
			statusCode:     http.StatusBadRequest,
			body:           `{"ok":false,"error_code":400,"description":"Bad Request: something new"}`,
			expectedReason: vo.DeliveryFailureUnreachable,
		},
		{
			name:           "the destination having trouble of its own",
			statusCode:     http.StatusInternalServerError,
			body:           `{"ok":false,"error_code":500,"description":"Internal Server Error"}`,
			expectedReason: vo.DeliveryFailureUnreachable,
		},
		{
			name:           "an answer that cannot be read at all",
			statusCode:     http.StatusOK,
			body:           `<html>maintenance</html>`,
			expectedReason: vo.DeliveryFailureUnreachable,
		},
		{
			name:           "an answer that contradicts its own status",
			statusCode:     http.StatusOK,
			body:           `{"ok":false,"description":"who knows"}`,
			expectedReason: vo.DeliveryFailureUnreachable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proxy := telegramAnswering(t, testCase.statusCode, testCase.body)

			reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

			require.NoError(t, err,
				"對方拒絕是一次問到答案的詢問，不是這一側壞掉")
			assert.Equal(t, testCase.expectedReason, reason)
		})
	}
}

// "It is slow today" and "it is not there" lead somebody to wait different lengths
// of time before deciding something is wrong.
func TestTelegramMessageDeliveryProxyTellsRunningOutOfTimeApartFromNotBeingThere(t *testing.T) {
	// slowTelegram answers, but later than anybody is willing to wait. It sleeps
	// rather than blocking on the request being cancelled, because a handler
	// waiting for that never learns the client gave up — and the test server
	// refuses to shut down while a handler is still running.
	slowTelegram := func(t *testing.T) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {
				time.Sleep(300 * time.Millisecond)
				_, _ = writer.Write([]byte(`{"ok":true}`))
			}))
		t.Cleanup(server.Close)

		return server
	}

	t.Run("an answer that never comes in time", func(t *testing.T) {
		server := slowTelegram(t)

		proxy := messaging.NewTelegramMessageDeliveryProxy(
			server.URL, &http.Client{Timeout: 30 * time.Millisecond})

		reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

		require.NoError(t, err)
		assert.Equal(t, vo.DeliveryFailureTimedOut, reason)
	})

	t.Run("a caller who gave up before the answer came", func(t *testing.T) {
		server := slowTelegram(t)

		proxy := messaging.NewTelegramMessageDeliveryProxy(server.URL, server.Client())
		executionContext, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()

		reason, err := proxy.Deliver(executionContext, aCredential, "哈囉")

		require.NoError(t, err)
		assert.Equal(t, vo.DeliveryFailureTimedOut, reason)
	})

	t.Run("nothing listening at all", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(writer http.ResponseWriter, request *http.Request) {}))
		address := server.URL
		server.Close()

		proxy := messaging.NewTelegramMessageDeliveryProxy(
			address, &http.Client{Timeout: time.Second})

		reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

		require.NoError(t, err)
		assert.Equal(t, vo.DeliveryFailureUnreachable, reason)
	})
}

// Telegram puts the bot token in the path, so an address written into an error is a
// token written into a log file — the likeliest way a secret ever escapes.
func TestTelegramMessageDeliveryProxyNeverPutsTheAddressInWhatItReturns(t *testing.T) {
	proxy := messaging.NewTelegramMessageDeliveryProxy(
		"http://127.0.0.1:1/never-listening", &http.Client{Timeout: 200 * time.Millisecond})

	reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

	require.NoError(t, err)
	assert.Equal(t, vo.DeliveryFailureUnreachable, reason)
	assert.False(t, strings.Contains(string(reason), "AAHqwertyuiop"))
}

// An address nothing can build a request out of is this side failing, not the
// destination refusing — and the refusal it reports must still not carry the token.
func TestTelegramMessageDeliveryProxyReportsItsOwnFailureWithoutTheToken(t *testing.T) {
	proxy := messaging.NewTelegramMessageDeliveryProxy("://not-an-address", http.DefaultClient)

	reason, err := proxy.Deliver(t.Context(), aCredential, "哈囉")

	require.Error(t, err)
	assert.Equal(t, vo.DeliveryFailureUnreachable, reason)
	assert.NotContains(t, err.Error(), "AAHqwertyuiop1234")
	assert.NotContains(t, err.Error(), "not-an-address")
}
