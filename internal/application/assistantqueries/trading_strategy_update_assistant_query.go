package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// TradingStrategyUpdateAssistantQuery lets the assistant change a set of rules it or
// the person already built.
//
// This is the half of the loop that makes replaying worth anything. Reading a report
// card and saying "the return is 3%" is a remark; reading it, loosening the sell
// condition and replaying again is the work. Without this the assistant could only
// ever build once and then describe what it would have changed.
//
// A rewrite replaces everything the trading strategy remembers, which is why the
// assistant is told to read one before changing it: sending only the part it wants to
// change would silently drop the rest.
//
// The refusal that matters most here is the one about running bots. A set of rules
// swapped out underneath a bot mid-round leaves nobody able to say which version that
// round used, so the rewrite is refused and the running bots are named. The assistant
// relays that rather than working around it — stopping somebody's bot is their call.
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

// Run rewrites the trading strategy and hands it back as stored.
func (tradingStrategyUpdateAssistantQuery *TradingStrategyUpdateAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	writeArguments := tradingStrategyWriteAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &writeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	tradingStrategyDto, updateError := tradingStrategyUpdateAssistantQuery.tradingStrategyApplication.
		UpdateTradingStrategy(
			executionContext, viewerID, writeArguments.ToWriteDto(writeArguments.TradingStrategyID))
	if updateError != nil {
		return "", updateError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}
