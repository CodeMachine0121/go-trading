package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// TradingStrategyListAssistantQuery lets the assistant see which sets of rules the
// person already has.
//
// It exists for the conversation that picks up where one left off — "carry on with
// the one we built yesterday" names nothing the assistant can act on until it can
// find out what that was.
type TradingStrategyListAssistantQuery struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyListAssistantQuery(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyListAssistantQuery {
	return &TradingStrategyListAssistantQuery{tradingStrategyApplication: tradingStrategyApplication}
}

func (tradingStrategyListAssistantQuery *TradingStrategyListAssistantQuery) Name() string {
	return "list_trading_strategies"
}

func (tradingStrategyListAssistantQuery *TradingStrategyListAssistantQuery) Description() string {
	return "列出使用者自己的每一份交易策略：識別碼、名稱、交易模式、來源代號與各來源的彙總刻度。" +
		"不含條件樹與參數值——要看完整內容請用 get_trading_strategy 指名一份。" +
		"刻度帶在這裡，是為了讓你一眼看出哪一份的來源刻度不一致、因而重演不了；" +
		"交易模式帶在這裡，是為了讓使用者說「我的帳戶不能放空」時，你一眼看出該去改哪幾份。"
}

func (tradingStrategyListAssistantQuery *TradingStrategyListAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{},"additionalProperties":false}`
}

// tradingStrategyDigest is a trading strategy as it appears in a list: enough to pick
// one by, and enough to see which ones cannot be replayed, without the trees.
type tradingStrategyDigest struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	SourceLabels []string `json:"sourceLabels"`
	// AggregationIntervals is every coarseness this one's sources read, in source
	// order. More than one distinct value means it cannot be replayed — saying so
	// here saves reading the whole thing only to be refused later.
	AggregationIntervals []string `json:"aggregationIntervals"`
}

// Run hands over, in brief, every trading strategy the person who asked owns.
// Holding none is an answer, not a refusal.
func (tradingStrategyListAssistantQuery *TradingStrategyListAssistantQuery) Run(
	executionContext context.Context, viewerID uint, _ string,
) (string, error) {
	tradingStrategyDtos, listError := tradingStrategyListAssistantQuery.tradingStrategyApplication.
		ListTradingStrategies(executionContext, viewerID)
	if listError != nil {
		return "", listError
	}

	digests := make([]tradingStrategyDigest, 0, len(tradingStrategyDtos))
	for _, tradingStrategyDto := range tradingStrategyDtos {
		sourceLabels := make([]string, 0, len(tradingStrategyDto.SignalSources))
		aggregationIntervals := make([]string, 0, len(tradingStrategyDto.SignalSources))
		for _, signalSource := range tradingStrategyDto.SignalSources {
			sourceLabels = append(sourceLabels, signalSource.Label)
			aggregationIntervals = append(aggregationIntervals, signalSource.AggregationInterval)
		}

		digests = append(digests, tradingStrategyDigest{
			ID:                   tradingStrategyDto.ID,
			Name:                 tradingStrategyDto.Name,
			SourceLabels:         sourceLabels,
			AggregationIntervals: aggregationIntervals,
		})
	}

	payload, marshalError := json.Marshal(struct {
		TradingStrategies []tradingStrategyDigest `json:"tradingStrategies"`
	}{TradingStrategies: digests})
	if marshalError != nil {
		return "", fmt.Errorf("render trading strategies: %w", marshalError)
	}

	return string(payload), nil
}
