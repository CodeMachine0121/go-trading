package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyUpdateAssistantQuery lets the assistant rewrite a trading strategy, which
// replaces every field (so it must read first) and is refused while bots run it, which it relays.
type TradingStrategyUpdateAssistantQuery struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyUpdateAssistantQuery(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyUpdateAssistantQuery {
	return &TradingStrategyUpdateAssistantQuery{tradingStrategyApplication: tradingStrategyApplication}
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Name() string {
	return "update_trading_strategy"
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Description() string {
	return "改寫一份既有的交易策略：名稱、信號來源、買入與賣出條件全部整包覆蓋。" +
		"改之前一定要先用 get_trading_strategy 讀它，只送要改的那一部分會把其餘的洗掉。" +
		"只要有任何一台機器人正在跑這份交易策略就會被拒絕，並告訴你是哪幾台——" +
		"這時候要把這件事轉達給使用者請他先停，不要想辦法繞過。"
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":` +
		`{"tradingStrategyId":{"type":"integer","description":"要改哪一份交易策略"},` +
		tradingStrategyWriteArgumentSchema +
		`},"required":["tradingStrategyId","name","signalSources","buyCondition","sellCondition"],` +
		`"additionalProperties":false}`
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	writeArguments := tradingStrategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, updateError := tradingStrategyUpdateAssistantQuery.tradingStrategyApplication.
		UpdateTradingStrategy(
			executionContext, origin.ViewerID, writeArguments.ToWriteDto(writeArguments.TradingStrategyID))
	if updateError != nil {
		return "", updateError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}
