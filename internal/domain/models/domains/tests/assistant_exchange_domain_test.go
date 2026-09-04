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

func TestAssistantExchangeRequestPutsTheQuestionAfterWhatCameBefore(t *testing.T) {
	request := anExchange(8).Request()

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "上次問的", request.Messages[0].Content)
	assert.Equal(t, "上次答的", request.Messages[1].Content)
	assert.Equal(t, "BTCUSDT 最近走勢如何", request.Messages[2].Content)
	assert.Equal(t, vo.AssistantMessageRoleAsk, request.Messages[2].Role)
	assert.Equal(t, 2000, request.AnswerLengthLimit)
	assert.False(t, request.QueryLimitReached)
	assert.Empty(t, request.Exchanges)
}

func TestAssistantExchangeShowsTheAssistantWhatItHasAlreadyLookedAt(t *testing.T) {
	exchange := anExchange(8).RecordQuery(aCall("list_trading_symbols"), `{"symbols":["BTCUSDT"]}`, false)

	request := exchange.Request()

	require.Len(t, request.Exchanges, 1)
	assert.Equal(t, "list_trading_symbols", request.Exchanges[0].Call.Name)
	assert.Equal(t, `{"symbols":["BTCUSDT"]}`, request.Exchanges[0].Outcome)
	assert.False(t, request.Exchanges[0].Rejected)
}

func TestAssistantExchangeTellsTheAssistantWhenItsQueriesAreSpent(t *testing.T) {
	// Being told is what turns a half answer into an honest one: the assistant knows
	// to speak with what it has instead of asking for more and getting nothing.
	exchange := anExchange(1).RecordQuery(aCall("list_trading_symbols"), "{}", false)

	assert.False(t, exchange.AllowsQuery())
	assert.True(t, exchange.Request().QueryLimitReached)
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
		RecordQuery(aCall("list_trading_symbols"), `{"symbols":["BTCUSDT"]}`, false).
		RecordQuery(aCall("get_k_candle_series"), "彙總刻度只接受 5m、15m、1h、4h、1d", true)

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
		RecordQuery(aCall("list_trading_symbols"), "{}", false).
		RecordQuery(aCall("get_k_candles"), "{}", false)

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

	recorded := exchange.RecordUsage(100).RecordQuery(aCall("list_trading_symbols"), "{}", false)

	assert.Empty(t, exchange.Request().Exchanges)
	assert.Equal(t, 0, exchange.ToTurn("x", time.Now()).Usage)
	assert.Len(t, recorded.Request().Exchanges, 1)
}
