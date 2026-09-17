package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// TradingStrategyCreateAssistantQuery lets the assistant put the scripts it writes
// together into one set of rules.
//
// Until it had this, the assistant could hand over an algorithm and nothing more: the
// thing a person actually runs is several scripts plus a buy tree and a sell tree,
// and assembling that was work it could describe but not do. Every conversation ended
// at "now go to the workbench and build it yourself".
//
// Saving is offered and deleting is not, for a sharper version of the reason that
// applies to a strategy script: a set of rules is several scripts and two trees that
// took a few rounds to get right, and rebuilding it costs far more than renaming one
// saved by mistake.
//
// Every rule a person's own save obeys is obeyed here — the name must be free, the
// labels must be declared before a condition names one, a source may only set knobs
// its script declared — because they arrive at the same model. A refusal is handed
// back to the assistant as the reason, and refusals here are expected rather than
// exceptional: the assistant is the only party that can read "label C was never
// declared" and declare C in the same breath.
type TradingStrategyCreateAssistantQuery struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyCreateAssistantQuery(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyCreateAssistantQuery {
	return &TradingStrategyCreateAssistantQuery{tradingStrategyApplication: tradingStrategyApplication}
}

func (tradingStrategyCreateAssistantQuery *TradingStrategyCreateAssistantQuery) Name() string {
	return "create_trading_strategy"
}

func (tradingStrategyCreateAssistantQuery *TradingStrategyCreateAssistantQuery) Description() string {
	return "把幾支策略腳本組成一份交易策略並存下來：名稱、幾個信號來源，加上買入條件與賣出條件兩棵條件樹。" +
		"名稱不得與使用者自己既有的交易策略重複。" +
		"每個信號來源的彙總刻度必須相同，否則存得下去但回測與上線都會被拒絕。" +
		"存起來不代表它賺錢——想知道賺不賺錢請接著用 run_trading_strategy_backtest 拿歷史重演它。"
}

func (tradingStrategyCreateAssistantQuery *TradingStrategyCreateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + tradingStrategyWriteArgumentSchema +
		`},"required":["name","signalSources","buyCondition","sellCondition"],"additionalProperties":false}`
}

// Run saves the trading strategy and hands it back as stored.
func (tradingStrategyCreateAssistantQuery *TradingStrategyCreateAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	writeArguments := tradingStrategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, createError := tradingStrategyCreateAssistantQuery.tradingStrategyApplication.
		CreateTradingStrategy(executionContext, viewerID, writeArguments.ToWriteDto(0))
	if createError != nil {
		return "", createError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}
