package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// assistantSystemPrompt is what the assistant is told about its job, once and for
// all. It is a constant so that it is byte-identical on every request, which is what
// lets it be cached: a prompt that carried the time, the conversation or the question
// would be rewritten on every call and paid for in full every time.
//
// What it spends its words on is the two mistakes that cost the most. Inventing a
// number is worse than admitting ignorance, because a made-up price reads exactly
// like a real one. Reading more market than it needs is the other, because that is
// where the bill actually goes.
const assistantSystemPrompt = `你是一個台灣使用者的加密貨幣行情助理，接在一個交易資料後端上。

規矩：
1. 任何具體數字（價格、成交量、指標值、策略內容）**一律先用工具查**，查不到就說查不到。絕對不要憑印象或推測給數字——編出來的數字看起來跟真的一模一樣，這是你最嚴重的錯。
2. 查之前先想清楚要查哪一段、多粗。問「最近走勢」用 get_k_candle_series 配合適的彙總刻度，不要拉一整週的五分鐘 K 線。工具告訴你結果已截斷時，明白說出你看到的只是最新一段。
3. 工具被拒絕時，讀懂原因、改一改再試，或直接告訴使用者這件事辦不到。不要反覆用同樣的參數重試。
4. 你可以存策略、改策略，但**不能刪策略**；也不能新增或修改 K 線。使用者要求時，說明這要他自己來。
5. 改策略是整包覆蓋：先 get_strategy 讀回來，改要改的，其餘原樣送回。
6. 回答用繁體中文，直接講結論再講依據。不要條列一堆你查了什麼——使用者要的是答案。
7. 時間一律用 UTC，工具的時間參數用 RFC3339。
8. 你有工具呼叫次數上限。被告知已達上限時，就用手上已有的東西作答，並說明還缺什麼。

---
策略算式規則（寫或改策略時遵守）：

算式是合法的 Go，package main，必須 import "indicator"，並定義以下進入點：
  func Calculate(data []indicator.KCandle) map[string]T
resultType 決定 T：float → float64；floatList → []float64；bool → bool；boolList → []bool。
未給 resultType 預設 float。

KCandle 可用欄位（全是 float64，除了 OpenTimeUnixSeconds 是 int64）：
  Open、High、Low、Close、Volume、QuoteVolume、TakerBuyBaseVolume、TakerBuyQuoteVolume、OpenTimeUnixSeconds

只能 import "math" 和 "sort"。不可存取 I/O、網路、時鐘、隨機數。

取參數（名稱必須與策略宣告完全相符，拼錯會讓算式當場失敗）：
  indicator.LookbackCount("名稱") → int    // 回看根數
  indicator.Number("名稱")        → float64 // 任意數字
  indicator.Boolean("名稱")       → bool    // 0=false，非零=true

參數種類（kind）只有三個：lookbackCount、number、boolean。

---
回測訊號規則（用 calculate_indicator 或直接跑回測時遵守）：

結果 map 的 key "signal" 是唯一的交易指令：正數 → 買進（開多）；負數 → 賣出（開空）；0 / 不存在 / NaN / Inf → 持平不動。
成交在當根 K 線收盤，這根訊號這根成交。
positionSizingMode：allIn（預設，全押）；percentage（需給 1–100 的百分比值）；fixedAmount（需給正數金額）。`

// queryLimitReachedNote is what the assistant is told once its queries are spent. It
// is appended to the last message rather than added to the instructions above,
// because the instructions are the cached part: changing them mid-exchange would
// throw the cache away on the one round trip that already has the most to carry.
const queryLimitReachedNote = "【系統】本次回答的工具查詢次數已用盡，不會再執行任何查詢。" +
	"請就目前已取得的資料作答，並明白說出你還缺什麼、因此結論到什麼程度為止。"

// ClaudeAssistantProxy asks Claude once and reports what came back.
//
// It is the only file that knows an assistant SDK exists. Everything it decides is a
// technical decision — which model, how hard it may think, where the cache breakpoint
// sits, how long to wait — and every business rule about what an exchange may cost
// lives outside it, in the domain, where it can be tested without paying for an
// answer.
//
// It runs no capability and never loops. One call in, one reply out: either an answer
// or a list of capabilities it wants run first.
type ClaudeAssistantProxy struct {
	client         anthropic.Client
	model          string
	effort         anthropic.OutputConfigEffort
	requestTimeout time.Duration
}

// NewClaudeAssistantProxy builds the proxy. An empty base address means the
// assistant's own, which is what it is in normal use.
func NewClaudeAssistantProxy(
	apiKey string, model string, effort string, baseUrl string, requestTimeout time.Duration,
) *ClaudeAssistantProxy {
	clientOptions := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseUrl != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(baseUrl))
	}

	return &ClaudeAssistantProxy{
		client:         anthropic.NewClient(clientOptions...),
		model:          model,
		effort:         anthropic.OutputConfigEffort(effort),
		requestTimeout: requestTimeout,
	}
}

// Reply asks Claude once.
//
// The wait is bounded here rather than left to the caller, because being too slow and
// being unreachable are the same thing to whoever is waiting for an answer, and both
// have to leave nothing behind.
func (claudeAssistantProxy *ClaudeAssistantProxy) Reply(
	executionContext context.Context, request vo.AssistantTurnRequestVo,
) (vo.AssistantReplyVo, error) {
	boundedContext, releaseWait := context.WithTimeout(executionContext, claudeAssistantProxy.requestTimeout)
	defer releaseWait()

	tools, toolsError := claudeAssistantProxy.toolsFor(request.Declarations)
	if toolsError != nil {
		return vo.AssistantReplyVo{}, toolsError
	}

	message, replyError := claudeAssistantProxy.client.Messages.New(boundedContext, anthropic.MessageNewParams{
		Model:     anthropic.Model(claudeAssistantProxy.model),
		MaxTokens: int64(request.AnswerLengthLimit),
		// The instructions and the capabilities are the same bytes on every request,
		// and they are rendered before the messages, so one breakpoint at the end of
		// them is what turns the largest fixed part of every exchange into a cache
		// read instead of a fresh charge.
		System: []anthropic.TextBlockParam{{
			Text:         assistantSystemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		OutputConfig: anthropic.OutputConfigParam{Effort: claudeAssistantProxy.effort},
		Tools:        tools,
		Messages:     claudeAssistantProxy.messagesFor(request),
	})
	if replyError != nil {
		return vo.AssistantReplyVo{}, fmt.Errorf("ask assistant: %w", replyError)
	}

	return claudeAssistantProxy.replyOf(message), nil
}

// toolSchema is a capability's arguments once read out of the text they are declared
// as. The loose value type is what a JSON schema is, and it is required by the SDK;
// it stays confined to this infrastructure file.
type toolSchema struct {
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}

// toolsFor turns the capabilities into what the SDK offers the assistant.
//
// A schema that cannot be read is a fault in this system rather than something the
// assistant did, so it stops the round trip instead of being handed over half formed:
// an assistant offered a capability it cannot call correctly will keep calling it
// incorrectly, and pay for every attempt.
func (claudeAssistantProxy *ClaudeAssistantProxy) toolsFor(
	declarations []vo.AssistantQueryDeclarationVo,
) ([]anthropic.ToolUnionParam, error) {
	tools := make([]anthropic.ToolUnionParam, 0, len(declarations))
	for _, declaration := range declarations {
		schema := toolSchema{}
		if unmarshalError := json.Unmarshal([]byte(declaration.ArgumentSchema), &schema); unmarshalError != nil {
			return nil, fmt.Errorf("read argument schema of %s: %w", declaration.Name, unmarshalError)
		}

		tool := anthropic.ToolParam{
			Name:        declaration.Name,
			Description: anthropic.String(declaration.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: schema.Properties,
				Required:   schema.Required,
			},
		}

		tools = append(tools, anthropic.ToolUnionParam{OfTool: &tool})
	}

	return tools, nil
}

// messagesFor lays out the conversation as the assistant sees it: the recent messages
// with the question last, then each round of lookups as the assistant's turn and the
// reply carrying its results.
//
// **A round is replayed whole.** Whatever the assistant said on the way goes back in
// the same turn as the requests it made, and all of that round's results come back in
// a single reply. Both halves matter: without the sentence, the assistant's next turn
// continues from a thought it can no longer see; and results split one per reply
// teach it that asking for several lookups at once does not work, which costs a round
// trip per lookup from then on.
func (claudeAssistantProxy *ClaudeAssistantProxy) messagesFor(
	request vo.AssistantTurnRequestVo,
) []anthropic.MessageParam {
	messages := make([]anthropic.MessageParam, 0, len(request.Messages)+len(request.Rounds)*2)

	for _, message := range request.Messages {
		if message.Role == vo.AssistantMessageRoleAnswer {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(message.Content)))
			continue
		}

		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(message.Content)))
	}

	for _, round := range request.Rounds {
		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(round.Exchanges)+1)
		if round.Narration != "" {
			assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(round.Narration))
		}

		resultBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(round.Exchanges))

		for _, exchange := range round.Exchanges {
			assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(
				exchange.Call.CallID,
				json.RawMessage(exchange.Call.Arguments),
				exchange.Call.Name,
			))
			resultBlocks = append(resultBlocks, anthropic.NewToolResultBlock(
				exchange.Call.CallID, exchange.Outcome, exchange.Rejected,
			))
		}

		messages = append(messages,
			anthropic.NewAssistantMessage(assistantBlocks...),
			anthropic.NewUserMessage(resultBlocks...))
	}

	if request.QueryLimitReached && len(messages) > 0 {
		lastMessage := &messages[len(messages)-1]
		lastMessage.Content = append(lastMessage.Content, anthropic.NewTextBlock(queryLimitReachedNote))
	}

	return messages
}

// replyOf reads what came back: the text as the answer, the tool requests as
// capabilities to run.
//
// Usage counts every kind of token the round trip touched, cached ones included. They
// are billed at different rates, but the daily allowance is a ceiling on how much
// assistant a day may consume, and a ceiling that could not see two thirds of the
// tokens would be a ceiling in name only.
func (claudeAssistantProxy *ClaudeAssistantProxy) replyOf(message *anthropic.Message) vo.AssistantReplyVo {
	answer := ""
	queryCalls := make([]vo.AssistantQueryCallVo, 0)

	for _, block := range message.Content {
		switch contentBlock := block.AsAny().(type) {
		case anthropic.TextBlock:
			answer += contentBlock.Text
		case anthropic.ToolUseBlock:
			queryCalls = append(queryCalls, vo.AssistantQueryCallVo{
				CallID:    contentBlock.ID,
				Name:      contentBlock.Name,
				Arguments: string(contentBlock.Input),
			})
		}
	}

	return vo.AssistantReplyVo{
		Answer:     answer,
		QueryCalls: queryCalls,
		Usage: int(message.Usage.InputTokens +
			message.Usage.OutputTokens +
			message.Usage.CacheCreationInputTokens +
			message.Usage.CacheReadInputTokens),
	}
}
