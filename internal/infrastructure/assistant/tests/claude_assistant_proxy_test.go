package assistant_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/assistant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentRequestBody is the request the proxy actually put on the wire, only as far as
// these tests look into it. What is asserted here is the layout of one round trip:
// get it wrong and the assistant answers a conversation that never happened.
type sentRequestBody struct {
	Model        string `json:"model"`
	MaxTokens    int    `json:"max_tokens"`
	OutputConfig struct {
		Effort string `json:"effort"`
	} `json:"output_config"`
	System []struct {
		Text         string          `json:"text"`
		CacheControl json.RawMessage `json:"cache_control"`
	} `json:"system"`
	Tools []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	} `json:"tools"`
	Messages []struct {
		Role    string `json:"role"`
		Content []struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			ID        string          `json:"id"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
			IsError   bool            `json:"is_error"`
		} `json:"content"`
	} `json:"messages"`
}

type assistantUnderTest struct {
	assistantProxy *assistant.ClaudeAssistantProxy
	sentRequest    *sentRequestBody
}

// newAssistantUnderTest points the proxy at a stand-in that records what it was sent
// and answers with the given reply.
func newAssistantUnderTest(
	t *testing.T, statusCode int, responseBody string, responseDelay time.Duration,
) assistantUnderTest {
	sentRequest := &sentRequestBody{}

	standIn := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			body, readError := io.ReadAll(request.Body)
			require.NoError(t, readError)
			require.NoError(t, json.Unmarshal(body, sentRequest))

			time.Sleep(responseDelay)
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(statusCode)
			_, _ = writer.Write([]byte(responseBody))
		}))
	t.Cleanup(standIn.Close)

	requestTimeout := 5 * time.Second
	if responseDelay > 0 {
		requestTimeout = responseDelay / 10
	}

	return assistantUnderTest{
		assistantProxy: assistant.NewClaudeAssistantProxy(
			"test-key", "claude-opus-5", "low", standIn.URL, requestTimeout),
		sentRequest: sentRequest,
	}
}

// aTurnRequest is one round trip's worth of input.
func aTurnRequest() vo.AssistantTurnRequestVo {
	return vo.AssistantTurnRequestVo{
		Messages: []vo.AssistantMessageVo{
			{Role: vo.AssistantMessageRoleAsk, Content: "上次問的"},
			{Role: vo.AssistantMessageRoleAnswer, Content: "上次答的"},
			{Role: vo.AssistantMessageRoleAsk, Content: "BTCUSDT 最近走勢如何"},
		},
		Declarations: []vo.AssistantQueryDeclarationVo{{
			Name:        "list_trading_symbols",
			Description: "列出交易標的",
			ArgumentSchema: `{"type":"object","properties":` +
				`{"symbol":{"type":"string"}},"required":["symbol"]}`,
		}},
		AnswerLengthLimit: 2000,
	}
}

const answeredResponse = `{"id":"msg_1","type":"message","role":"assistant",` +
	`"model":"claude-opus-5","content":[{"type":"text","text":"最近在盤整"}],` +
	`"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":50,` +
	`"cache_creation_input_tokens":30,"cache_read_input_tokens":20}}`

func TestClaudeAssistantProxyAsksWithWhatItWasGiven(t *testing.T) {
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	reply, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.NoError(t, replyError)
	assert.Equal(t, "最近在盤整", reply.Answer)
	assert.Empty(t, reply.QueryCalls)
	// Every kind of token the round trip touched is counted. A ceiling that could not
	// see the cached ones would be a ceiling in name only.
	assert.Equal(t, 200, reply.Usage)

	assert.Equal(t, "claude-opus-5", fixture.sentRequest.Model)
	assert.Equal(t, 2000, fixture.sentRequest.MaxTokens)
	assert.Equal(t, "low", fixture.sentRequest.OutputConfig.Effort)
}

func TestClaudeAssistantProxyCachesTheOnePartThatNeverChanges(t *testing.T) {
	// The instructions are the same bytes on every request and are rendered before the
	// messages, so one breakpoint at the end of them turns the largest fixed part of
	// every exchange into a cache read instead of a fresh charge.
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	_, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.NoError(t, replyError)
	require.Len(t, fixture.sentRequest.System, 1)
	assert.NotEmpty(t, fixture.sentRequest.System[0].Text)
	assert.NotEmpty(t, fixture.sentRequest.System[0].CacheControl)
}

func TestClaudeAssistantProxyOffersEachCapabilityWithArgumentsItCanRead(t *testing.T) {
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	_, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.NoError(t, replyError)
	require.Len(t, fixture.sentRequest.Tools, 1)
	assert.Equal(t, "list_trading_symbols", fixture.sentRequest.Tools[0].Name)
	assert.Equal(t, "列出交易標的", fixture.sentRequest.Tools[0].Description)

	schema := struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}{}
	require.NoError(t, json.Unmarshal(fixture.sentRequest.Tools[0].InputSchema, &schema))
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "symbol")
	assert.Equal(t, []string{"symbol"}, schema.Required)
}

func TestClaudeAssistantProxyLaysOutTheConversationInTurns(t *testing.T) {
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	_, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.NoError(t, replyError)
	require.Len(t, fixture.sentRequest.Messages, 3)
	assert.Equal(t, "user", fixture.sentRequest.Messages[0].Role)
	assert.Equal(t, "上次問的", fixture.sentRequest.Messages[0].Content[0].Text)
	assert.Equal(t, "assistant", fixture.sentRequest.Messages[1].Role)
	assert.Equal(t, "上次答的", fixture.sentRequest.Messages[1].Content[0].Text)
	assert.Equal(t, "user", fixture.sentRequest.Messages[2].Role)
	assert.Equal(t, "BTCUSDT 最近走勢如何", fixture.sentRequest.Messages[2].Content[0].Text)
}

func TestClaudeAssistantProxyGivesEveryLookupItsResult(t *testing.T) {
	// What the assistant needs is that every request it made has a result attached.
	// A request left unanswered is what the assistant's own API refuses outright.
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	request := aTurnRequest()
	request.Rounds = []vo.AssistantQueryRoundVo{
		{
			Exchanges: []vo.AssistantQueryExchangeVo{{
				Call:    vo.AssistantQueryCallVo{CallID: "call_1", Name: "list_trading_symbols", Arguments: `{"symbol":"BTCUSDT"}`},
				Outcome: `{"symbols":["BTCUSDT"]}`,
			}},
		},
		{
			Exchanges: []vo.AssistantQueryExchangeVo{{
				Call:     vo.AssistantQueryCallVo{CallID: "call_2", Name: "list_trading_symbols", Arguments: `{}`},
				Outcome:  "彙總刻度只接受 5m、15m、1h、4h、1d",
				Rejected: true,
			}},
		},
	}

	_, replyError := fixture.assistantProxy.Reply(t.Context(), request)

	require.NoError(t, replyError)
	require.Len(t, fixture.sentRequest.Messages, 7)

	assert.Equal(t, "assistant", fixture.sentRequest.Messages[3].Role)
	assert.Equal(t, "tool_use", fixture.sentRequest.Messages[3].Content[0].Type)
	assert.Equal(t, "call_1", fixture.sentRequest.Messages[3].Content[0].ID)
	assert.JSONEq(t, `{"symbol":"BTCUSDT"}`, string(fixture.sentRequest.Messages[3].Content[0].Input))

	assert.Equal(t, "user", fixture.sentRequest.Messages[4].Role)
	assert.Equal(t, "tool_result", fixture.sentRequest.Messages[4].Content[0].Type)
	assert.Equal(t, "call_1", fixture.sentRequest.Messages[4].Content[0].ToolUseID)
	assert.Contains(t, string(fixture.sentRequest.Messages[4].Content[0].Content), "BTCUSDT")
	assert.False(t, fixture.sentRequest.Messages[4].Content[0].IsError)

	assert.Equal(t, "call_2", fixture.sentRequest.Messages[6].Content[0].ToolUseID)
	assert.True(t, fixture.sentRequest.Messages[6].Content[0].IsError)
}

func TestClaudeAssistantProxyTellsTheAssistantWhenItsQueriesAreSpent(t *testing.T) {
	// The note rides on the last message rather than the instructions, because the
	// instructions are the cached part: rewriting them here would throw the cache away
	// on the one round trip that already carries the most.
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	request := aTurnRequest()
	request.QueryLimitReached = true

	_, replyError := fixture.assistantProxy.Reply(t.Context(), request)

	require.NoError(t, replyError)
	lastMessage := fixture.sentRequest.Messages[len(fixture.sentRequest.Messages)-1]
	require.Len(t, lastMessage.Content, 2)
	assert.Contains(t, lastMessage.Content[1].Text, "查詢次數已用盡")
	for _, systemBlock := range fixture.sentRequest.System {
		assert.NotContains(t, systemBlock.Text, "查詢次數已用盡")
	}
}

func TestClaudeAssistantProxyReportsWhichCapabilitiesTheAssistantWants(t *testing.T) {
	fixture := newAssistantUnderTest(t, http.StatusOK,
		`{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5",`+
			`"content":[{"type":"text","text":"我先查一下"},`+
			`{"type":"tool_use","id":"call_9","name":"list_trading_symbols","input":{"symbol":"BTCUSDT"}}],`+
			`"stop_reason":"tool_use","usage":{"input_tokens":100,"output_tokens":50}}`, 0)

	reply, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.NoError(t, replyError)
	assert.Equal(t, "我先查一下", reply.Answer)
	require.Len(t, reply.QueryCalls, 1)
	assert.Equal(t, "call_9", reply.QueryCalls[0].CallID)
	assert.Equal(t, "list_trading_symbols", reply.QueryCalls[0].Name)
	assert.JSONEq(t, `{"symbol":"BTCUSDT"}`, reply.QueryCalls[0].Arguments)
	assert.Equal(t, 150, reply.Usage)
}

func TestClaudeAssistantProxyReportsAnAssistantThatWouldNotAnswer(t *testing.T) {
	fixture := newAssistantUnderTest(t, http.StatusInternalServerError, `{"error":"boom"}`, 0)

	_, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.Error(t, replyError)
	assert.Contains(t, replyError.Error(), "ask assistant")
}

func TestClaudeAssistantProxyStopsWaitingAfterTheTimeAllowed(t *testing.T) {
	// Being too slow and being unreachable are the same thing to whoever is waiting,
	// and the wait is bounded here so both leave nothing behind.
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 500*time.Millisecond)

	_, replyError := fixture.assistantProxy.Reply(t.Context(), aTurnRequest())

	require.Error(t, replyError)
	assert.Contains(t, replyError.Error(), "ask assistant")
}

func TestClaudeAssistantProxyRefusesToOfferACapabilityItCannotDescribe(t *testing.T) {
	// An assistant offered a capability it cannot call correctly will keep calling it
	// incorrectly and pay for every attempt, so a broken schema stops the round trip
	// instead of being handed over half formed.
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	request := aTurnRequest()
	request.Declarations = []vo.AssistantQueryDeclarationVo{
		{Name: "broken", Description: "壞掉的", ArgumentSchema: `not a schema`},
	}

	_, replyError := fixture.assistantProxy.Reply(t.Context(), request)

	require.Error(t, replyError)
	assert.Contains(t, replyError.Error(), "broken")
}

func TestClaudeAssistantProxyReplaysOneRoundAsOneTurn(t *testing.T) {
	// 一輪裡問了三件事，就要以**一則助手訊息帶三個請求**、**一則回覆帶三個結果**送回去。
	// 拆成三輪，助手會學到「一次問幾件事沒有用」，從此每件事都多花一次往返。
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	request := aTurnRequest()
	request.Rounds = []vo.AssistantQueryRoundVo{{
		Narration: "我先看一下系統裡既有策略的算式寫法。",
		Exchanges: []vo.AssistantQueryExchangeVo{
			{Call: vo.AssistantQueryCallVo{CallID: "call_1", Name: "list_strategies", Arguments: `{}`}, Outcome: "{}"},
			{Call: vo.AssistantQueryCallVo{CallID: "call_2", Name: "list_trading_symbols", Arguments: `{}`}, Outcome: "{}"},
			{Call: vo.AssistantQueryCallVo{CallID: "call_3", Name: "get_k_candles", Arguments: `{}`}, Outcome: "{}"},
		},
	}}

	_, replyError := fixture.assistantProxy.Reply(t.Context(), request)

	require.NoError(t, replyError)
	// 三則近期訊息，加上這一輪的助手訊息與回覆——一共五則，不是八則。
	require.Len(t, fixture.sentRequest.Messages, 5)

	assistantTurn := fixture.sentRequest.Messages[3]
	assert.Equal(t, "assistant", assistantTurn.Role)
	// 那句旁白在最前面，三個請求接在後面：助手下一輪才看得到自己剛才在想什麼。
	require.Len(t, assistantTurn.Content, 4)
	assert.Equal(t, "text", assistantTurn.Content[0].Type)
	assert.Equal(t, "我先看一下系統裡既有策略的算式寫法。", assistantTurn.Content[0].Text)
	assert.Equal(t, "tool_use", assistantTurn.Content[1].Type)
	assert.Equal(t, "tool_use", assistantTurn.Content[3].Type)

	resultTurn := fixture.sentRequest.Messages[4]
	assert.Equal(t, "user", resultTurn.Role)
	require.Len(t, resultTurn.Content, 3)
	assert.Equal(t, "call_1", resultTurn.Content[0].ToolUseID)
	assert.Equal(t, "call_3", resultTurn.Content[2].ToolUseID)
}

func TestClaudeAssistantProxyLeavesOutANarrationThatWasNotThere(t *testing.T) {
	// 助手什麼都沒說就直接要查的時候，不要替它插一則空白的文字區塊——
	// 那是它自己的 API 會拒絕的形狀。
	fixture := newAssistantUnderTest(t, http.StatusOK, answeredResponse, 0)

	request := aTurnRequest()
	request.Rounds = []vo.AssistantQueryRoundVo{{
		Exchanges: []vo.AssistantQueryExchangeVo{
			{Call: vo.AssistantQueryCallVo{CallID: "call_1", Name: "list_strategies", Arguments: `{}`}, Outcome: "{}"},
		},
	}}

	_, replyError := fixture.assistantProxy.Reply(t.Context(), request)

	require.NoError(t, replyError)
	assistantTurn := fixture.sentRequest.Messages[3]
	require.Len(t, assistantTurn.Content, 1)
	assert.Equal(t, "tool_use", assistantTurn.Content[0].Type)
}
