package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// TradingSymbolListAssistantQuery lets the assistant find out which markets the
// system knows about.
//
// It is the capability the assistant reaches for first, because every other one needs
// a market named. Without it the assistant would have to guess at names, and a guessed
// name comes back empty in a way that looks exactly like a market with no data.
type TradingSymbolListAssistantQuery struct {
	tradingSymbolApplication *application.TradingSymbolApplication
}

func NewTradingSymbolListAssistantQuery(
	tradingSymbolApplication *application.TradingSymbolApplication,
) *TradingSymbolListAssistantQuery {
	return &TradingSymbolListAssistantQuery{tradingSymbolApplication: tradingSymbolApplication}
}

func (tradingSymbolListAssistantQuery *TradingSymbolListAssistantQuery) Name() string {
	return "list_trading_symbols"
}

func (tradingSymbolListAssistantQuery *TradingSymbolListAssistantQuery) Description() string {
	return "列出系統認得的每一個交易標的（市場代號，例如 BTCUSDT）。" +
		"任何要查行情或算指標的問題都先用這個確認代號存在，不要自己猜代號。"
}

func (tradingSymbolListAssistantQuery *TradingSymbolListAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{},"additionalProperties":false}`
}

// Run hands over every market the system knows about. Holding none is an answer, not
// a refusal — a freshly built system genuinely knows of none.
func (tradingSymbolListAssistantQuery *TradingSymbolListAssistantQuery) Run(
	executionContext context.Context, _ string,
) (string, error) {
	tradingSymbolDtos, listError := tradingSymbolListAssistantQuery.tradingSymbolApplication.ListTradingSymbols(
		executionContext)
	if listError != nil {
		return "", listError
	}

	symbols := make([]string, 0, len(tradingSymbolDtos))
	for _, tradingSymbolDto := range tradingSymbolDtos {
		symbols = append(symbols, tradingSymbolDto.Symbol)
	}

	payload, marshalError := json.Marshal(struct {
		Symbols []string `json:"symbols"`
	}{Symbols: symbols})
	if marshalError != nil {
		return "", fmt.Errorf("render trading symbols: %w", marshalError)
	}

	return string(payload), nil
}
