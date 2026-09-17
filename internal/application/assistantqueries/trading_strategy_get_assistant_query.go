package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// tradingStrategyGetAssistantArguments is what the assistant sends to read one
// trading strategy.
type tradingStrategyGetAssistantArguments struct {
	TradingStrategyID uint `json:"tradingStrategyId"`
}

// TradingStrategyGetAssistantQuery lets the assistant read one set of rules in full.
//
// Reading it in full is what makes changing it possible: a rewrite replaces
// everything a trading strategy remembers, so an assistant asked to loosen one
// condition has to know the rest before it can send them back unchanged.
//
// Somebody else's comes back as the system's own words for "not found", word for word
// the same as one that does not exist — which is what stops the identifier becoming a
// way to discover whose rules are whose.
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

// Run hands over the trading strategy in full.
func (tradingStrategyGetAssistantQuery *TradingStrategyGetAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	getArguments := tradingStrategyGetAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &getArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, findError := tradingStrategyGetAssistantQuery.tradingStrategyApplication.
		GetTradingStrategy(executionContext, viewerID, getArguments.TradingStrategyID)
	if findError != nil {
		return "", findError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}
