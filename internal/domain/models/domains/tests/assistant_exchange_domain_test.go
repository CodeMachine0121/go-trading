package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const theTurnID = uint(7)

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
	// 思考文字要跟查詢請求一起送回，否則助手下一輪會從看不到的想法接續。
	exchange := anExchange(8).RecordRound(
		"我先看一下系統裡既有策略腳本的算式寫法。",
		aRound(anOutcome("list_strategy_scripts", `{"strategyScripts":[]}`, false)))

	request := exchange.Request()

	require.Len(t, request.Rounds, 1)
	assert.Equal(t, "我先看一下系統裡既有策略腳本的算式寫法。", request.Rounds[0].Narration)
}

func TestAssistantExchangeKeepsOneRoundsLookupsTogether(t *testing.T) {
	// 一輪問三件事仍是一輪，拆開送回會教助手不要一次多問。
	exchange := anExchange(8).RecordRound("一次查三件", aRound(
		anOutcome("list_trading_symbols", "{}", false),
		anOutcome("list_strategy_scripts", "{}", false),
		anOutcome("get_k_candles", "{}", false)))

	request := exchange.Request()

	require.Len(t, request.Rounds, 1)
	assert.Len(t, request.Rounds[0].Exchanges, 3)
}

func TestAssistantExchangeCountsEveryLookupInARound(t *testing.T) {
	// 查詢次數按次計，不按輪計。
	exchange := anExchange(8).RecordRound("", aRound(
		anOutcome("a", "{}", false),
		anOutcome("b", "{}", false),
		anOutcome("c", "{}", false)))

	assert.Len(t, exchange.AllowedCalls([]vo.AssistantQueryCallVo{
		aCall("d"), aCall("e"), aCall("f"), aCall("g"), aCall("h"), aCall("i"),
	}), 5)
	assert.Equal(t, 3, exchange.ToAnsweredTurn(theTurnID, "答完了").QueryCount)
}

func TestAssistantExchangeRecordsNothingForARoundThatLookedAtNothing(t *testing.T) {
	// 沒有查詢的一輪不留紀錄。
	exchange := anExchange(8).RecordRound("只是說說話", aRound())

	assert.Empty(t, exchange.Request().Rounds)
	assert.Equal(t, 0, exchange.ToAnsweredTurn(theTurnID, "答完了").QueryCount)
}

func TestAssistantExchangeTellsTheAssistantWhenItsQueriesAreSpent(t *testing.T) {
	// Telling the assistant lets it answer with what it has instead of asking again.
	exchange := anExchange(1).RecordRound("", aRound(
		anOutcome("list_trading_symbols", "{}", false)))

	assert.Empty(t, exchange.AllowedCalls([]vo.AssistantQueryCallVo{aCall("get_k_candles")}))
	assert.True(t, exchange.Request().QueryLimitReached)
}

func TestAssistantExchangeAllowsOnlyAsManyLookupsAsItHasLeft(t *testing.T) {
	// 剩兩次卻要五次時，放行前兩次。
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
	// Lookup-only round trips still count toward usage.
	exchange := anExchange(8).RecordUsage(100).RecordUsage(150).RecordUsage(50)

	turn := exchange.ToAnsweredTurn(theTurnID, "答完了")

	assert.Equal(t, 300, turn.Usage)
}

func TestAssistantExchangeToStartedTurnReservesThePlaceTheAnswerWillGo(t *testing.T) {
	// Written before the assistant is asked, so an in-progress answer is visible and a restart can sweep it up.
	exchange := anExchange(8)

	turn := exchange.ToStartedTurn(time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC))

	assert.Equal(t, "BTCUSDT 最近走勢如何", turn.Ask)
	assert.Equal(t, string(vo.AssistantTurnRunning), turn.Status)
	assert.Empty(t, turn.Answer)
	assert.Equal(t, 0, turn.Usage)
	assert.Equal(t, time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC), turn.CreatedAt)
}

func TestAssistantExchangeToFailedTurnChargesNobodyForAnAnswerTheyNeverGot(t *testing.T) {
	// A failed exchange records zero usage so repeated failures can't spend someone's daily allowance without an answer.
	exchange := anExchange(8).RecordUsage(120).RecordRound("", aRound(
		anOutcome("list_trading_symbols", "{}", false)))

	turn := exchange.ToFailedTurn(theTurnID, "助手目前沒有回應，請稍後再試")

	assert.Equal(t, theTurnID, turn.ID)
	assert.Equal(t, string(vo.AssistantTurnFailed), turn.Status)
	assert.Equal(t, "助手目前沒有回應，請稍後再試", turn.FailureReason)
	assert.Equal(t, 0, turn.Usage)
	assert.Empty(t, turn.Answer)
	assert.Empty(t, turn.Queries)
}

func TestAssistantExchangeToAnsweredTurnIsWhatWillBeStored(t *testing.T) {
	exchange := anExchange(8).
		RecordUsage(120).
		RecordRound("", aRound(
			anOutcome("list_trading_symbols", `{"symbols":["BTCUSDT"]}`, false))).
		RecordRound("", aRound(
			anOutcome("get_k_candle_series", "彙總刻度只接受 5m、15m、1h、4h、1d", true)))

	turn := exchange.ToAnsweredTurn(theTurnID, "最近在盤整")

	assert.Equal(t, theTurnID, turn.ID)
	assert.Equal(t, "最近在盤整", turn.Answer)
	assert.Equal(t, string(vo.AssistantTurnAnswered), turn.Status)
	assert.Equal(t, 120, turn.Usage)
	assert.Equal(t, 2, turn.QueryCount)
	assert.False(t, turn.StoppedAtQueryLimit)
	// The question was settled when the exchange began and isn't rewritten.
	assert.Empty(t, turn.Ask)

	require.Len(t, turn.Queries, 2)
	assert.Equal(t, 1, turn.Queries[0].Sequence)
	assert.Equal(t, "list_trading_symbols", turn.Queries[0].QueryName)
	assert.False(t, turn.Queries[0].Rejected)
	assert.Equal(t, 2, turn.Queries[1].Sequence)
	assert.Equal(t, "彙總刻度只接受 5m、15m、1h、4h、1d", turn.Queries[1].Outcome)
	assert.True(t, turn.Queries[1].Rejected)
}

func TestAssistantExchangeToAnsweredTurnMarksAnAnswerThatRanOutOfQueries(t *testing.T) {
	// Recorded so an answer that stopped early is distinguishable from a poor one.
	exchange := anExchange(2).
		RecordRound("", aRound(anOutcome("list_trading_symbols", "{}", false))).
		RecordRound("", aRound(anOutcome("get_k_candles", "{}", false)))

	turn := exchange.ToAnsweredTurn(theTurnID, "只查到這些")

	assert.True(t, turn.StoppedAtQueryLimit)
	assert.Equal(t, 2, turn.QueryCount)
}

func TestAssistantExchangeToStartedTurnStoresTheMomentInUniversalTime(t *testing.T) {
	elsewhere := time.FixedZone("UTC+8", 8*60*60)

	turn := anExchange(8).ToStartedTurn(time.Date(2026, 9, 4, 18, 0, 0, 0, elsewhere))

	assert.Equal(t, time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC), turn.CreatedAt.UTC())
	assert.Equal(t, time.UTC, turn.CreatedAt.Location())
}

func TestAssistantExchangeLeavesTheValueItWasAskedFromAlone(t *testing.T) {
	exchange := anExchange(8)

	recorded := exchange.RecordUsage(100).RecordRound("", aRound(
		anOutcome("list_trading_symbols", "{}", false)))

	assert.Empty(t, exchange.Request().Rounds)
	assert.Equal(t, 0, exchange.ToAnsweredTurn(theTurnID, "x").Usage)
	assert.Len(t, recorded.Request().Rounds, 1)
}
