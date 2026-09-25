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

// assistantSystemPrompt is a constant so it is byte-identical on every request and therefore cacheable.
// where the bill actually goes.
const assistantSystemPrompt = `你是一個台灣使用者的加密貨幣行情助理，接在一個交易資料後端上。

規矩：
1. 任何具體數字（價格、成交量、指標值、策略腳本內容）**一律先用工具查**，查不到就說查不到。絕對不要憑印象或推測給數字——編出來的數字看起來跟真的一模一樣，這是你最嚴重的錯。
2. 查之前先想清楚要查哪一段、多粗。問「最近走勢」用 get_k_candle_series 配合適的彙總刻度，不要拉一整週的一分鐘 K 線。工具告訴你結果已截斷時，明白說出你看到的只是最新一段。
3. 工具被拒絕時，讀懂原因、改一改再試，或直接告訴使用者這件事辦不到。不要反覆用同樣的參數重試。
4. 你可以存策略腳本、改策略腳本，但**不能刪策略腳本**；也不能新增或修改 K 線。使用者要求時，說明這要他自己來。
5. 改策略腳本是整包覆蓋：先 get_strategy_script 讀回來，改要改的，其餘原樣送回。
6. 回答用繁體中文，直接講結論再講依據。不要條列一堆你查了什麼——使用者要的是答案。
7. 時間一律用 UTC，工具的時間參數用 RFC3339。
8. 你有工具呼叫次數上限。被告知已達上限時，就用手上已有的東西作答，並說明還缺什麼。

---
策略腳本算式規則（寫或改策略腳本時遵守）：

算式是合法的 Go，package main，必須 import "indicator"，並定義以下進入點：
  func Calculate(data []indicator.KCandle) map[string]T
resultType 決定 T：float → float64；floatList → []float64；bool → bool；boolList → []bool。
未給 resultType 預設 float。

resultType 給 signal 時進入點**換一個形狀**——不是 map，是單一個信號：
  func Calculate(data []indicator.KCandle) indicator.Signal
回傳值只能是 indicator.Buy、indicator.Sell、indicator.Hold 其中一個；不設方向會當場失敗。

**要被交易策略當信號來源的腳本，resultType 一律給 signal。** 交易策略的條件比對的是
買入／賣出／持有，只有 signal 這一種說得出那三個值；宣告成 float 卻回傳 indicator.Signal
的腳本存得進去、但接不上任何一份交易策略，而且跑起來就失敗。兩者必須一起改：
換進入點形狀時就換 resultType，換 resultType 時就換進入點形狀。

KCandle 可用欄位（全是 float64，除了 OpenTimeUnixSeconds 是 int64）：
  Open、High、Low、Close、Volume、QuoteVolume、TakerBuyBaseVolume、TakerBuyQuoteVolume、OpenTimeUnixSeconds

只能 import "math" 和 "sort"。不可存取 I/O、網路、時鐘、隨機數，也不可使用 goroutine（go 敘述）或 channel。

取參數（名稱必須與策略腳本宣告完全相符，拼錯會讓算式當場失敗）：
  indicator.LookbackCount("名稱") → int    // 回看根數
  indicator.Number("名稱")        → float64 // 任意數字
  indicator.Boolean("名稱")       → bool    // 0=false，非零=true

參數種類（kind）只有三個：lookbackCount、number、boolean。

---
回測訊號規則（用 calculate_indicator 或直接跑回測時遵守）：

結果 map 的 key "signal" 是唯一的交易指令：正數 → 買進；負數 → 賣出；0 / 不存在 / NaN / Inf → 持平不動。
成交在當根 K 線收盤，這根訊號這根成交。
positionSizingMode：allIn（預設，全押）；percentage（需給 1–100 的百分比值）；fixedAmount（需給正數金額）。
這個系統的重演只做現貨，而且只有這一種：買入時空手就開倉，已經有倉位就當作沒聽到；賣出就平倉把錢
收回來、之後空手等下一個買點；空手時賣出什麼都不做，永遠不開空倉。
沒有交易模式可以指定，也開不了槓桿——借錢、做空與強制平倉是合約帳戶的事，那是另外一件事，這裡做不到。
使用者提到要放空、要開槓桿、或說他在合約帳戶上操作時，直接告訴他這個系統目前只重演現貨，
不要替他改成別的設定去湊：湊出來的成績單是照他做不到的操作算的，而他不會發現。`

// queryLimitReachedNote is appended to the last message rather than the system prompt so the cached prefix stays intact.
const queryLimitReachedNote = "【系統】本次回答的工具查詢次數已用盡，不會再執行任何查詢。" +
	"請就目前已取得的資料作答，並明白說出你還缺什麼、因此結論到什麼程度為止。"

// ClaudeAssistantProxy makes one Claude call per Reply, returning either an answer or requested tool calls; cost rules live in the domain.
type ClaudeAssistantProxy struct {
	client         anthropic.Client
	model          string
	effort         anthropic.OutputConfigEffort
	requestTimeout time.Duration
}

// NewClaudeAssistantProxy uses the SDK's default endpoint when the base address is empty.
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

// Reply bounds the wait itself, since a slow assistant and an unreachable one must both leave nothing behind.
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
		// One cache breakpoint after the fixed system prompt and tools makes them a cache read on every request.
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

// toolSchema holds a parsed JSON schema; the loose value type is required by the SDK.
type toolSchema struct {
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}

// toolsFor fails the round trip on an unparseable schema instead of offering a tool the assistant cannot call correctly.
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

// messagesFor replays each lookup round whole, text and all tool calls in one assistant turn and all results in one reply, so the assistant keeps its reasoning and keeps batching lookups.
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

// replyOf counts cached tokens in usage too, because the daily allowance caps total consumption.
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
