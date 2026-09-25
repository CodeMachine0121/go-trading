package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyCreateAssistantQuery lets the assistant assemble scripts and buy/sell trees into
// a saved trading strategy; deleting is not offered, and domain refusals go back for it to fix.
type TradingStrategyCreateAssistantQuery struct {
	tradingStrategyApplication   *application.TradingStrategyApplication
	assistantRevisionApplication *application.AssistantRevisionApplication
}

func NewTradingStrategyCreateAssistantQuery(
	tradingStrategyApplication *application.TradingStrategyApplication,
	assistantRevisionApplication *application.AssistantRevisionApplication,
) *TradingStrategyCreateAssistantQuery {
	return &TradingStrategyCreateAssistantQuery{
		tradingStrategyApplication:   tradingStrategyApplication,
		assistantRevisionApplication: assistantRevisionApplication,
	}
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

func (tradingStrategyCreateAssistantQuery *TradingStrategyCreateAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	writeArguments := tradingStrategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, createError := tradingStrategyCreateAssistantQuery.tradingStrategyApplication.
		CreateTradingStrategy(executionContext, origin.ViewerID, writeArguments.ToWriteDto(0))
	if createError != nil {
		return "", createError
	}

	tradingStrategyCreateAssistantQuery.assistantRevisionApplication.RecordCreation(
		executionContext, origin, vo.AssistantRevisionSubjectTradingStrategy, tradingStrategyDto.ID)

	return renderedTradingStrategy(tradingStrategyDto)
}
