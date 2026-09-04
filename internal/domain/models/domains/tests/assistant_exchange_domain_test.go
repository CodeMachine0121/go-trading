package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anExchange is an exchange that has just started, so that each test advances exactly
// one thing about it.
func anExchange(queryLimit int) domains.AssistantExchangeDomain {
	return domains.NewAssistantExchangeDomain(
		"BTCUSDT 最近走勢如何",
		[]vo.AssistantMessageVo{
			{Role: vo.AssistantMessageRoleAsk, Content: "上次問的"},
			{Role: vo.AssistantMessageRoleAnswer, Content: "上次答的"},
		},
		[]vo.AssistantQueryDeclarationVo{{Name: "list_trading_symbols"}},
		queryLimit,
		2000,
	)
}

func aCall(name string) vo.AssistantQueryCallVo {
	return vo.AssistantQueryCallVo{CallID: "call_" + name, Name: name, Arguments: `{}`}
}

// aRound is one round trip's worth of lookups, each with what came back.
func aRound(outcomes ...vo.AssistantQueryExchangeVo) []vo.AssistantQueryExchangeVo {
	return outcomes
}

func anOutcome(name string, outcome string, rejected bool) vo.AssistantQueryExchangeVo {
	return vo.AssistantQueryExchangeVo{Call: aCall(name), Outcome: outcome, Rejected: rejected}
}

func TestAssistantExchangeRequestPutsTheQuestionAfterWhatCameBefore(t *testing.T) {
	request := anExchange(8).Request()

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "上次問的", request.Messages[0].Content)
	assert.Equal(t, "上次答的", request.Messages[1].Content)
	assert.Equal(t, "BTCUSDT 最近走勢如何", request.Messages[2].Content)
	assert.Equal(t, vo.AssistantMessageRoleAsk, request.Messages[2].Role)
	assert.Equal(t, 2000, request.AnswerLengthLimit)
	assert.False(t, request.QueryLimitReached)
	assert.Empty(t, request.Rounds)
}

func TestAssistantExchangeShowsTheAssistantWhatItHasAlreadyLookedAt(t *testing.T) {
	exchange := anExchange(8).RecordRound("", aRound(
		anOutcome("list_trading_symbols", `{"symbols":["BTCUSDT"]}`, false)))

	request := exchange.Request()

	require.Len(t, request.Rounds, 1)
	require.Len(t, request.Rounds[0].Exchanges, 1)
	assert.Equal(t, "list_trading_symbols", request.Rounds[0].Exchanges[0].Call.Name)
	assert.Equal(t, `{"symbols":["BTCUSDT"]}`, request.Rounds[0].Exchanges[0].Outcome)
	assert.False(t, request.Rounds[0].Exchanges[0].Rejected)
}

func TestAssistantExchangeKeepsWhatTheAssistantSaidOnTheWay(t *testing.T) {
	// 「我先看一下既有的算式寫法」那句話要跟著它的查詢請求一起回去，
	// 否則助手下一輪是從一個它已經看不到的想法往下接——答案會從半句話開始。
	exchange := anExchange(8).RecordRound(
		"我先看一下系統裡既有策略的算式寫法。",
		aRound(anOutcome("list_strategies", `{"strategies":[]}`, false)))

	request := exchange.Request()

	require.Len(t, request.Rounds, 1)
	assert.Equal(t, "我先看一下系統裡既有策略的算式寫法。", request.Rounds[0].Narration)
}

func TestAssistantExchangeKeepsOneRoundsLookupsTogether(t *testing.T) {
	// 一輪裡問了三件事就是一輪。拆成三輪送回去，助手會學到「一次問幾件事沒有用」，
	// 從此每件事都多花一次往返。
	exchange := anExchange(8).RecordRound("一次查三件", aRound(
		anOutcome("list_trading_symbols", "{}", false),
		anOutcome("list_strategies", "{}", false),
		anOutcome("get_k_candles", "{}", false)))

	request := exchange.Request()

	require.Len(t, request.Rounds, 1)
	assert.Len(t, request.Rounds[0].Exchanges, 3)
}

func TestAssistantExchangeCountsEveryLookupInARound(t *testing.T) {
	// 一輪三次就是三次，不是一次——不然一個回答可以在八輪裡查上幾十次。
	exchange := anExchange(8).RecordRound("", aRound(
		anOutcome("a", "{}", false),
		anOutcome("b", "{}", false),
		anOutcome("c", "{}", false)))

	assert.Len(t, exchange.AllowedCalls([]vo.AssistantQueryCallVo{
		aCall("d"), aCall("e"), aCall("f"), aCall("g"), aCall("h"), aCall("i"),
	}), 5)
	assert.Equal(t, 3, exchange.ToTurn("答完了", time.Now()).QueryCount)
}

func TestAssistantExchangeRecordsNothingForARoundThatLookedAtNothing(t *testing.T) {
	// 一輪沒有任何查詢，就不是一輪。留一個空的下來只會讓紀錄多一筆什麼都沒做的東西。
	exchange := anExchange(8).RecordRound("只是說說話", aRound())

	assert.Empty(t, exchange.Request().Rounds)
	assert.Equal(t, 0, exchange.ToTurn("答完了", time.Now()).QueryCount)
}

func TestAssistantExchangeTellsTheAssistantWhenItsQueriesAreSpent(t *testing.T) {
	// Being told is what turns a half answer into an honest one: the assistant knows
	// to speak with what it has instead of asking for more and getting nothing.
	exchange := anExchange(1).RecordRound("", aRound(
		anOutcome("list_trading_symbols", "{}", false)))

	assert.Empty(t, exchange.AllowedCalls([]vo.AssistantQueryCallVo{aCall("get_k_candles")}))
	assert.True(t, exchange.Request().QueryLimitReached)
}

func TestAssistantExchangeAllowsOnlyAsManyLookupsAsItHasLeft(t *testing.T) {
	// 助手一口氣要五次而只剩兩次時：全部拒掉是丟掉它有權做的事，
	// 全部放行則是讓上限不成為上限。誠實的答案是「前兩次」。
	exchange := anExchange(3).RecordRound("", aRound(anOutcome("a", "{}", false)))

	allowed := exchange.AllowedCalls([]vo.AssistantQueryCallVo{
		aCall("b"), aCall("c"), aCall("d"), aCall("e"),
	})

	require.Len(t, allowed, 2)
	assert.Equal(t, "b", allowed[0].Name)
	assert.Equal(t, "c", allowed[1].Name)
}

func TestAssistantExchangeAllowsEveryLookupWhenThereIsRoom(t *testing.T) {
	allowed := anExchange(8).AllowedCalls([]vo.AssistantQueryCallVo{aCall("a"), aCall("b")})

	assert.Len(t, allowed, 2)
}

func TestAssistantExchangeAddsUpWhatEveryRoundTripCost(t *testing.T) {
	// A round trip that only asked for a lookup was still paid for. An allowance that
	// could not see those trips would be one a long exchange walks straight through.
	exchange := anExchange(8).RecordUsage(100).RecordUsage(150).RecordUsage(50)

	turn := exchange.ToTurn("答完了", time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, 300, turn.Usage)
}

func TestAssistantExchangeToTurnIsWhatWillBeStored(t *testing.T) {
	exchange := anExchange(8).
		RecordUsage(120).
		RecordRound("", aRound(
			anOutcome("list_trading_symbols", `{"symbols":["BTCUSDT"]}`, false))).
		RecordRound("", aRound(
			anOutcome("get_k_candle_series", "彙總刻度只接受 5m、15m、1h、4h、1d", true)))

	turn := exchange.ToTurn("最近在盤整", time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC))

	assert.Equal(t, "BTCUSDT 最近走勢如何", turn.Ask)
	assert.Equal(t, "最近在盤整", turn.Answer)
	assert.Equal(t, 120, turn.Usage)
	assert.Equal(t, 2, turn.QueryCount)
	assert.False(t, turn.StoppedAtQueryLimit)
	assert.Equal(t, time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC), turn.CreatedAt)

	require.Len(t, turn.Queries, 2)
	assert.Equal(t, 1, turn.Queries[0].Sequence)
	assert.Equal(t, "list_trading_symbols", turn.Queries[0].QueryName)
	assert.False(t, turn.Queries[0].Rejected)
	assert.Equal(t, 2, turn.Queries[1].Sequence)
	assert.Equal(t, "彙總刻度只接受 5m、15m、1h、4h、1d", turn.Queries[1].Outcome)
	assert.True(t, turn.Queries[1].Rejected)
}

func TestAssistantExchangeToTurnMarksAnAnswerThatRanOutOfQueries(t *testing.T) {
	// An answer that stopped early is a different thing from a poor one, and the
	// record is the only place that difference survives.
	exchange := anExchange(2).
		RecordRound("", aRound(anOutcome("list_trading_symbols", "{}", false))).
		RecordRound("", aRound(anOutcome("get_k_candles", "{}", false)))

	turn := exchange.ToTurn("只查到這些", time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC))

	assert.True(t, turn.StoppedAtQueryLimit)
	assert.Equal(t, 2, turn.QueryCount)
}

func TestAssistantExchangeToTurnStoresTheMomentInUniversalTime(t *testing.T) {
	elsewhere := time.FixedZone("UTC+8", 8*60*60)

	turn := anExchange(8).ToTurn("答完了", time.Date(2026, 9, 4, 18, 0, 0, 0, elsewhere))

	assert.Equal(t, time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC), turn.CreatedAt.UTC())
	assert.Equal(t, time.UTC, turn.CreatedAt.Location())
}

func TestAssistantExchangeLeavesTheValueItWasAskedFromAlone(t *testing.T) {
	exchange := anExchange(8)

	recorded := exchange.RecordUsage(100).RecordRound("", aRound(
		anOutcome("list_trading_symbols", "{}", false)))

	assert.Empty(t, exchange.Request().Rounds)
	assert.Equal(t, 0, exchange.ToTurn("x", time.Now()).Usage)
	assert.Len(t, recorded.Request().Rounds, 1)
}
