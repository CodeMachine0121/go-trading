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

// tradingStrategyBacktestAssistantArguments is what the assistant sends to replay one
// trading strategy over a stretch of market that has already happened.
//
// There is no coarseness here, no algorithm and no trading mode, for the same reason
// a person's request carries none of them: the trading strategy already says all
// three, and a second answer would need a rule about which one wins.
//
// The money arrives as text and is parsed into an exact decimal. Sending it as a JSON
// number would hand the amounts a person stakes to a binary float, which is the one
// type this system refuses everywhere else money is involved.
type tradingStrategyBacktestAssistantArguments struct {
	TradingStrategyID uint      `json:"tradingStrategyId"`
	Symbol            string    `json:"symbol"`
	StartTime         time.Time `json:"startTime"`
	EndTime           time.Time `json:"endTime"`
	InitialCapital    string    `json:"initialCapital"`
	// PositionSizingMode is how much each opening stakes, and PositionSizingValue the
	// figure that goes with it. Staking everything needs no figure.
	PositionSizingMode  string `json:"positionSizingMode"`
	PositionSizingValue string `json:"positionSizingValue"`
	// StopLossPercentage and TakeProfitPercentage are the two exit distances this run
	// simulates, and EntryCostPercentage and ExitCostPercentage what the act of
	// trading costs at each end. All four are optional; left out, nothing is
	// simulated and nothing is charged.
	//
	// They are here because the assistant's whole job is a loop — build the rules,
	// replay, read the report card, adjust — and the two most valuable adjustments in
	// it are hanging a stop and putting the real fees in. An entry point missing them
	// would hand back a different report card from the one the person gets for the
	// same settings, with nothing on the page to say the difference came from which
	// door was used.
	StopLossPercentage   string `json:"stopLossPercentage"`
	TakeProfitPercentage string `json:"takeProfitPercentage"`
	EntryCostPercentage  string `json:"entryCostPercentage"`
	ExitCostPercentage   string `json:"exitCostPercentage"`
}

// ToRequestDto turns what the assistant declared into the shape the domain replays,
// reading each amount as an exact decimal.
//
// An amount that is not a number at all becomes zero rather than a failure here, and
// that is deliberate: zero capital is already refused by the replay itself, in words
// the assistant can act on, so parsing does not need a second vocabulary for the same
// mistake.
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

// mostRecentClosedTrades is the tail of the round trips, at most closedTradeLimit of
// them, plus the sentence that says so when there were more.
//
// The most recent rather than the first, because a replay is read backwards: what the
// strategy has been doing lately is what decides whether to adjust it. An empty
// sentence means nothing was left out.
func mostRecentClosedTrades(closedTrades []dto.ClosedTradeDto) ([]dto.ClosedTradeDto, string) {
	if len(closedTrades) <= closedTradeLimit {
		return closedTrades, ""
	}

	return closedTrades[len(closedTrades)-closedTradeLimit:], fmt.Sprintf(
		"這次重演共有 %d 筆交易，上面只列出最近的 %d 筆。成績單裡的數字是全部 %d 筆算出來的。",
		len(closedTrades), closedTradeLimit, len(closedTrades))
}

// decimalOrZero reads an amount, answering zero for anything it cannot read.
func decimalOrZero(amount string) decimal.Decimal {
	parsedAmount, parseError := decimal.NewFromString(amount)
	if parseError != nil {
		return decimal.Zero
	}

	return parsedAmount
}

// closedTradeLimit is how many round trips one replay hands the assistant.
//
// It exists for the same reason the K candle ceiling does, and it bites harder here:
// what a capability hands back is replayed to the assistant on **every subsequent
// round** of the same answer. An assistant that replays a chatty strategy four or
// five times — which is exactly what forty queries are for — would be carrying every
// trade of every attempt, and would run out of room to think before it ran out of
// queries.
//
// Fifty is enough to see how a strategy behaves: how long it holds, whether it wins
// in streaks, whether the losses are small. Reading all six hundred is not how that
// question gets answered, and the report card above already counts them all.
const closedTradeLimit = 50

// tradingStrategyBacktestReport is one replay as the assistant reads it: the report
// card and the round trips, and deliberately not the equity curve.
//
// It is its own shape rather than the result with a field hidden, because the curve
// is wanted on the HTTP path — a screen draws it. Marking it unsendable there to keep
// it from the assistant would break the drawing to fix the reading.
//
// What it leaves out costs nothing. A point per candle rendered as text is hundreds
// of numbers crowding out the report card, and every one of them can be worked back
// out from the round trips and the opening capital.
type tradingStrategyBacktestReport struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	// StartTime and EndTime are where the candles actually replayed begin and end,
	// which is not always what was asked for: an end reaching into an interval that
	// has not finished is pulled back to the last one that has.
	StartTime       time.Time              `json:"startTime"`
	EndTime         time.Time              `json:"endTime"`
	UsedCandleCount int                    `json:"usedCandleCount"`
	Summary         dto.BacktestSummaryDto `json:"summary"`
	// ClosedTrades holds the most recent round trips, at most closedTradeLimit of
	// them. ClosedTradesTruncated says so outright when there were more.
	//
	// Being told is the whole point. An assistant reading fifty of six hundred trades
	// without knowing it will describe a strategy that does not exist — and unlike a
	// truncated answer, nothing about the list itself gives that away.
	ClosedTrades          []dto.ClosedTradeDto `json:"closedTrades"`
	ClosedTradesTruncated string               `json:"closedTradesTruncated,omitempty"`
}

// TradingStrategyBacktestAssistantQuery lets the assistant find out what the rules it
// built would have done.
//
// This is the capability that closes the loop. Without it the assistant hands over an
// algorithm and goes blind: whether the thing it wrote makes money is a question it
// cannot answer, so the person has to run the replay and report back. With it, a
// single answer can build a set of rules, replay it, read that the return is three
// percent against a target of ten, loosen a condition and replay again.
//
// Nothing is stored, exactly as on the person's own path: the assistant will run this
// many times while it converges, and a table filling up with attempts nobody asked to
// keep is the cost of pretending otherwise.
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
		"交易模式也沒有可以給——那是這份交易策略自己記著的：longShort 永遠在市場裡，" +
		"賣出會把多倉平掉並在同一棒反手做空；spot 只做多，賣出就平倉把錢收回來、之後空手等下一個買點，" +
		"空手時聽到賣出什麼都不做。使用者說他的帳戶不能放空（台股現貨、ETF、多數券商帳戶）時，" +
		"要去改那份交易策略的交易模式（trading_strategy 的 tradingMode），不是在這裡指定——" +
		"用錯的那一個，成績單會是照他做不到的操作算出來的。" +
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

// Run replays the trading strategy and hands back the report card and the round
// trips.
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
