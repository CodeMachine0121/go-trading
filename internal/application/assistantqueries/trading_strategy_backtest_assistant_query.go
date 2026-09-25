package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// tradingStrategyBacktestAssistantArguments carries no interval, algorithm or trading mode
// because the trading strategy defines them; amounts arrive as text so money is never a float.
type tradingStrategyBacktestAssistantArguments struct {
	TradingStrategyID uint      `json:"tradingStrategyId"`
	Symbol            string    `json:"symbol"`
	StartTime         time.Time `json:"startTime"`
	EndTime           time.Time `json:"endTime"`
	InitialCapital    string    `json:"initialCapital"`
	// PositionSizingValue goes with PositionSizingMode; staking everything needs no figure.
	PositionSizingMode  string `json:"positionSizingMode"`
	PositionSizingValue string `json:"positionSizingValue"`
	// StopLossPercentage, TakeProfitPercentage and the two cost percentages are optional (left out,
	// nothing is simulated or charged) and mirror the person's path so both get the same report card.
	StopLossPercentage   string `json:"stopLossPercentage"`
	TakeProfitPercentage string `json:"takeProfitPercentage"`
	EntryCostPercentage  string `json:"entryCostPercentage"`
	ExitCostPercentage   string `json:"exitCostPercentage"`
}

// ToRequestDto parses each amount as an exact decimal; an unparsable amount becomes zero because
// the replay itself already refuses zero capital in words the assistant can act on.
func (arguments tradingStrategyBacktestAssistantArguments) ToRequestDto() dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:               arguments.Symbol,
		StartTime:            arguments.StartTime,
		EndTime:              arguments.EndTime,
		InitialCapital:       decimalOrZero(arguments.InitialCapital),
		PositionSizingMode:   arguments.PositionSizingMode,
		PositionSizingValue:  decimalOrZero(arguments.PositionSizingValue),
		StopLossPercentage:   decimalOrZero(arguments.StopLossPercentage),
		TakeProfitPercentage: decimalOrZero(arguments.TakeProfitPercentage),
		EntryCostPercentage:  decimalOrZero(arguments.EntryCostPercentage),
		ExitCostPercentage:   decimalOrZero(arguments.ExitCostPercentage),
	}
}

// mostRecentClosedTrades keeps the last closedTradeLimit round trips, since recent behaviour
// decides adjustments, plus a notice when trades were left out (empty when none were).
func mostRecentClosedTrades(closedTrades []dto.ClosedTradeDto) ([]dto.ClosedTradeDto, string) {
	if len(closedTrades) <= closedTradeLimit {
		return closedTrades, ""
	}

	return closedTrades[len(closedTrades)-closedTradeLimit:], fmt.Sprintf(
		"這次重演共有 %d 筆交易，上面只列出最近的 %d 筆。成績單裡的數字是全部 %d 筆算出來的。",
		len(closedTrades), closedTradeLimit, len(closedTrades))
}

func decimalOrZero(amount string) decimal.Decimal {
	parsedAmount, parseError := decimal.NewFromString(amount)
	if parseError != nil {
		return decimal.Zero
	}

	return parsedAmount
}

// closedTradeLimit caps round trips per replay because each result is replayed to the assistant
// on every later round, and fifty is enough to judge behaviour while the summary counts them all.
const closedTradeLimit = 50

// tradingStrategyBacktestReport is its own shape so the assistant gets the report card and round
// trips without the equity curve, which the HTTP path still needs for drawing.
type tradingStrategyBacktestReport struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	// StartTime and EndTime are the replayed range, with an end in an unfinished interval pulled back.
	StartTime       time.Time              `json:"startTime"`
	EndTime         time.Time              `json:"endTime"`
	UsedCandleCount int                    `json:"usedCandleCount"`
	Summary         dto.BacktestSummaryDto `json:"summary"`
	// ClosedTradesTruncated says outright when older trades were dropped, since nothing in the list
	// itself would reveal it.
	ClosedTrades          []dto.ClosedTradeDto `json:"closedTrades"`
	ClosedTradesTruncated string               `json:"closedTradesTruncated,omitempty"`
}

// TradingStrategyBacktestAssistantQuery lets the assistant replay the rules it built and iterate
// on them; nothing is stored, since it will run many times while converging.
type TradingStrategyBacktestAssistantQuery struct {
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication
}

func NewTradingStrategyBacktestAssistantQuery(
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication,
) *TradingStrategyBacktestAssistantQuery {
	return &TradingStrategyBacktestAssistantQuery{
		tradingStrategyBacktestApplication: tradingStrategyBacktestApplication,
	}
}

func (tradingStrategyBacktestAssistantQuery *TradingStrategyBacktestAssistantQuery) Name() string {
	return "run_trading_strategy_backtest"
}

func (tradingStrategyBacktestAssistantQuery *TradingStrategyBacktestAssistantQuery) Description() string {
	return "拿一段已經發生過的歷史，把一份交易策略從頭重演一遍，交回成績單與每一筆進出場。" +
		"沒有彙總刻度可以給——那是這份交易策略的信號來源自己說的，而且每個來源必須一致，不一致會整次拒絕。" +
		"交易明細只給最近 50 筆，超過時會明講；成績單裡的數字一律是全部交易算出來的。" +
		"**這個系統的重演只做現貨**：買入時空手就開倉，已經有倉位就當作沒聽到；" +
		"賣出就平倉把錢收回來、之後空手等下一個買點；空手時聽到賣出什麼都不做。" +
		"沒有交易模式可以給，也沒有槓桿可以開——借錢、做空與強制平倉是合約帳戶的事，" +
		"那是另外一件事，這裡做不到。使用者提到要放空或上槓桿時，直接說這個系統目前只重演現貨，" +
		"不要替他改成別的設定去湊。" +
		"成績單裡的「打架棒數」(conflictedCandleCount) 一定要看：它是買入與賣出同時成立的棒數，" +
		"那幾棒一律不動作。兩百棒裡打架一百八十棒的交易策略，成績單會很漂亮（幾乎沒有交易），" +
		"但那代表它根本沒有在做決定，不是它很穩。" +
		"止損止盈（stopLossPercentage／takeProfitPercentage）不給就不模擬，" +
		"成績單會是照「一路抱到訊號叫你走」算出來的。" +
		"交易成本（entryCostPercentage／exitCostPercentage）不給就當交易免費——那會讓成績單偏樂觀，" +
		"而且交易越頻繁偏得越多：台股一趟進出約 0.47%，一年兩百趟光成本就吃掉六成本金。" +
		"使用者問「扣掉手續費還賺嗎」，或在比較兩支交易頻率差很多的策略時，一定要把費率填進去再跑一次。" +
		"成績單的 totalTransactionCost 是這次總共付掉多少；每一筆交易的 profit 已經是扣掉成本後的淨額，" +
		"勝率也是照淨額算的。" +
		"重演的是過去，不是對未來的保證；結果不留存，每次問都重算一遍。"
}

func (tradingStrategyBacktestAssistantQuery *TradingStrategyBacktestAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"tradingStrategyId":{"type":"integer","description":"要重演哪一份交易策略"},` +
		`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"},` +
		`"startTime":{"type":"string","description":"從哪一刻開始，ISO 8601 世界標準時間，例如 2026-09-10T00:00:00Z"},` +
		`"endTime":{"type":"string","description":"到哪一刻為止，ISO 8601 世界標準時間。指向未來時讀作現在"},` +
		`"initialCapital":{"type":"string","description":"手上一開始有多少錢，以字串給精確數字（例如 \"500000\"），必須大於零"},` +
		`"positionSizingMode":{"type":"string","enum":["allIn","percentage","fixedAmount"],` +
		`"description":"每次開倉押多少：allIn 全押（不必給 positionSizingValue）、percentage 押帳戶的百分之幾、fixedAmount 每次押固定金額"},` +
		`"positionSizingValue":{"type":"string","description":"配合 positionSizingMode 的數字，以字串給（percentage 給 0 到 100、fixedAmount 給金額）；allIn 時不必給"},` +
		`"stopLossPercentage":{"type":"string","description":"止損價離進場價幾個百分點，以字串給（例如 \"2\"）。不給就不模擬止損；0 到 100"},` +
		`"takeProfitPercentage":{"type":"string","description":"止盈價離進場價幾個百分點，以字串給。不給就不模擬止盈；0 到 100"},` +
		`"entryCostPercentage":{"type":"string","description":"開倉付的手續費，佔押注金額的百分之幾，以字串給（台股手續費六折約 \"0.0855\"、幣安約 \"0.1\"）。不給就當交易免費；0 到 100"},` +
		`"exitCostPercentage":{"type":"string","description":"平倉付的手續費與稅，佔成交金額的百分之幾，以字串給（台股六折含證交稅約 \"0.3855\"）。不給就跟 entryCostPercentage 一樣；0 到 100"}` +
		`},"required":["tradingStrategyId","symbol","startTime","endTime","initialCapital","positionSizingMode"],` +
		`"additionalProperties":false}`
}

func (tradingStrategyBacktestAssistantQuery *TradingStrategyBacktestAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	backtestArguments := tradingStrategyBacktestAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &backtestArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	resultDto, replayError := tradingStrategyBacktestAssistantQuery.tradingStrategyBacktestApplication.
		RunTradingStrategyBacktest(
			executionContext, viewerID, backtestArguments.TradingStrategyID,
			backtestArguments.ToRequestDto())
	if replayError != nil {
		return "", replayError
	}

	closedTrades, truncationNotice := mostRecentClosedTrades(resultDto.ClosedTrades)

	payload, marshalError := json.Marshal(tradingStrategyBacktestReport{
		Symbol:                resultDto.Symbol,
		Interval:              resultDto.Interval,
		StartTime:             resultDto.StartTime,
		EndTime:               resultDto.EndTime,
		UsedCandleCount:       resultDto.UsedCandleCount,
		Summary:               resultDto.Summary,
		ClosedTrades:          closedTrades,
		ClosedTradesTruncated: truncationNotice,
	})
	if marshalError != nil {
		return "", fmt.Errorf("render backtest report: %w", marshalError)
	}

	return string(payload), nil
}
