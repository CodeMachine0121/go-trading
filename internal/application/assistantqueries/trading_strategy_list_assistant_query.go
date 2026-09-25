package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyListAssistantQuery lets the assistant find the person's existing trading
// strategies, e.g. to resume "the one we built yesterday".
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

// tradingStrategyDigest lists a trading strategy without its trees.
type tradingStrategyDigest struct {
	ID           uint     `json:"id"`
	Name         string   `json:"name"`
	SourceLabels []string `json:"sourceLabels"`
	// AggregationIntervals lists each source's interval in order; more than one distinct value means
	// the strategy cannot be replayed.
	AggregationIntervals []string `json:"aggregationIntervals"`
}

// Run lists every trading strategy the asker owns; holding none is an answer, not a refusal.
func (tradingStrategyListAssistantQuery *TradingStrategyListAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, _ string,
) (string, error) {
	tradingStrategyDtos, listError := tradingStrategyListAssistantQuery.tradingStrategyApplication.
		ListTradingStrategies(executionContext, origin.ViewerID)
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
