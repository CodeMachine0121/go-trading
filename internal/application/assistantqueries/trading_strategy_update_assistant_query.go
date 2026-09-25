package assistantqueries

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyUpdateAssistantQuery lets the assistant rewrite a trading strategy, which replaces every field
// (so it must read first); a strategy the person already had is only proposed, for the person to confirm.
type TradingStrategyUpdateAssistantQuery struct {
	assistantRevisionApplication *application.AssistantRevisionApplication
}

func NewTradingStrategyUpdateAssistantQuery(
	assistantRevisionApplication *application.AssistantRevisionApplication,
) *TradingStrategyUpdateAssistantQuery {
	return &TradingStrategyUpdateAssistantQuery{assistantRevisionApplication: assistantRevisionApplication}
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Name() string {
	return "update_trading_strategy"
}

func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Description() string {
	return "改寫一份既有的交易策略：名稱、信號來源、買入與賣出條件全部整包覆蓋。" +
		"改之前一定要先用 get_trading_strategy 讀它，只送要改的那一部分會把其餘的洗掉。" +
		"**這段對話裡你自己剛建立、而且沒有任何機器人用到的**交易策略會直接改好；" +
		"其餘的（使用者本來就有的、別段對話建立的、有機器人在用的）只會變成一筆「等使用者確認的修改」，" +
		"在使用者按下確認之前一個字都不會變——回傳會明講 pending，這時候要告訴使用者請他確認，" +
		"不要說已經改好，也不要拿它還沒改的版本去重演並當成改過的結果。" +
		"確認時如果有機器人正在跑這份交易策略，確認會被擋下，使用者得先停掉它們。"
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
	return tradingStrategyUpdateAssistantQuery.assistantRevisionApplication.Revise(
		executionContext, origin, vo.AssistantRevisionSubjectTradingStrategy, arguments)
}
