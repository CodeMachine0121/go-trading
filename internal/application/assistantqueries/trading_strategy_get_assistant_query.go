package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

type tradingStrategyGetAssistantArguments struct {
	TradingStrategyID uint `json:"tradingStrategyId"`
}

// TradingStrategyGetAssistantQuery hands over a trading strategy in full so a rewrite can send the
// rest back unchanged; someone else's reads exactly like a missing one, so IDs reveal no owners.
type TradingStrategyGetAssistantQuery struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyGetAssistantQuery(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyGetAssistantQuery {
	return &TradingStrategyGetAssistantQuery{tradingStrategyApplication: tradingStrategyApplication}
}

func (tradingStrategyGetAssistantQuery *TradingStrategyGetAssistantQuery) Name() string {
	return "get_trading_strategy"
}

func (tradingStrategyGetAssistantQuery *TradingStrategyGetAssistantQuery) Description() string {
	return "以識別碼讀一份交易策略的完整內容：每個信號來源（代號、指名哪支腳本、彙總刻度、參數值）" +
		"與買入、賣出兩棵條件樹。要改一份交易策略之前一定要先讀，因為 update_trading_strategy 是整包覆蓋。"
}

func (tradingStrategyGetAssistantQuery *TradingStrategyGetAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"tradingStrategyId":{"type":"integer","description":"交易策略識別碼"}` +
		`},"required":["tradingStrategyId"],"additionalProperties":false}`
}

func (tradingStrategyGetAssistantQuery *TradingStrategyGetAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	getArguments := tradingStrategyGetAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &getArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, findError := tradingStrategyGetAssistantQuery.tradingStrategyApplication.
		GetTradingStrategy(executionContext, origin.ViewerID, getArguments.TradingStrategyID)
	if findError != nil {
		return "", findError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}
