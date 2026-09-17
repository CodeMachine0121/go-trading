package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// askedAt is the moment every test asks at, so that the day it falls in — and
// therefore the stretch usage is summed over — is the same in all of them.
var askedAt = time.Date(2026, 9, 4, 13, 45, 10, 0, time.UTC)

var (
	dayStart = time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	dayEnd   = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
)

// theQueryName is the one capability the service under test is given. Which
// capabilities exist is the composition root's decision, so a test only needs one to
// prove that the offered set is what gets reached.
const theQueryName = "list_trading_symbols"

// startedTurnID is the exchange every accepted question reserves. Which row it is
// does not matter to any case here — only that the answer is written back over that
// one.
const startedTurnID = uint(77)

type assistantConversationServiceUnderTest struct {
	assistantConversationService *service.AssistantConversationService
	conversationRepository       *mocks.MockIConversationRepository
	assistantProxy               *mocks.MockIAssistantProxy
	assistantQuery               *mocks.MockIAssistantQuery
	// completedTurns carries whatever was written back over the reserved row. The
	// answer is written from a goroutine the ask does not wait on, so a case has to
	// wait for it rather than read straight after asking.
	completedTurns chan entities.AssistantTurn
}

// newAssistantConversationServiceUnderTest wires the service with every ceiling it
// obeys. The two that tests vary are arguments; the rest are the defaults, so that a
// test about one ceiling is not also a test about another.
func newAssistantConversationServiceUnderTest(
	t *testing.T, queryLimit int, dailyUsageAllowance int,
) assistantConversationServiceUnderTest {
	mockController := gomock.NewController(t)
	conversationRepository := mocks.NewMockIConversationRepository(mockController)
	assistantProxy := mocks.NewMockIAssistantProxy(mockController)
	assistantQuery := mocks.NewMockIAssistantQuery(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)

	assistantQuery.EXPECT().Name().Return(theQueryName).AnyTimes()
	assistantQuery.EXPECT().Description().Return("列出交易標的").AnyTimes()
	assistantQuery.EXPECT().ArgumentSchema().Return(`{"type":"object"}`).AnyTimes()
	clockProxy.EXPECT().Now().Return(askedAt).AnyTimes()

	// Every ending is captured, whichever it is. Cases that expect one wait for it;
	// cases refused before anything started never produce one.
	completedTurns := make(chan entities.AssistantTurn, 1)
	conversationRepository.EXPECT().CompleteTurn(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, turn entities.AssistantTurn) error {
			completedTurns <- turn

			return nil
		}).AnyTimes()

	return assistantConversationServiceUnderTest{
		assistantConversationService: service.NewAssistantConversationService(
			conversationRepository,
			assistantProxy,
			[]domaininterface.IAssistantQuery{assistantQuery},
			clockProxy,
			20,
			queryLimit,
			dailyUsageAllowance,
			2000,
		),
		conversationRepository: conversationRepository,
		assistantProxy:         assistantProxy,
		assistantQuery:         assistantQuery,
		completedTurns:         completedTurns,
	}
}

// expectNewConversation accepts a question that named no conversation, storing it as
// the first exchange of a new one, and hands back what was stored for a case to read.
func (fixture assistantConversationServiceUnderTest) expectNewConversation(
	conversationID uint,
) *entities.Conversation {
	startedConversation := &entities.Conversation{}

	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, conversation entities.Conversation,
		) (entities.Conversation, error) {
			*startedConversation = conversation
			conversation.ID = conversationID
			conversation.Turns[0].ID = startedTurnID

			return conversation, nil
		})

	return startedConversation
}

// expectAppendedTurn accepts a question aimed at a conversation that already exists,
// and hands back the exchange that was reserved for its answer.
func (fixture assistantConversationServiceUnderTest) expectAppendedTurn(
	conversationID uint,
) *entities.AssistantTurn {
	startedTurn := &entities.AssistantTurn{}

	fixture.conversationRepository.EXPECT().AppendTurn(gomock.Any(), conversationID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, id uint, turn entities.AssistantTurn,
		) (entities.Conversation, error) {
			*startedTurn = turn
			turn.ID = startedTurnID

			return entities.Conversation{ID: id, Turns: []entities.AssistantTurn{turn}}, nil
		})

	return startedTurn
}

// awaitCompletedTurn waits for the answer to be written back over the reserved row.
//
// The wait is real rather than a peek at some flag: the answer is written from a
// goroutine the ask deliberately does not wait on, so a case reading straight after
// asking would be reading a row nothing has touched yet. Two seconds is far longer
// than any of these need and short enough to fail rather than hang.
func (fixture assistantConversationServiceUnderTest) awaitCompletedTurn(
	t *testing.T,
) entities.AssistantTurn {
	t.Helper()

	select {
	case completedTurn := <-fixture.completedTurns:
		return completedTurn
	case <-time.After(2 * time.Second):
		t.Fatal("答案沒有被寫回那一列——它應該在連線之外跑完再補上")

		return entities.AssistantTurn{}
	}
}

// expectUsageToday says what has been spent today. Every ask reads it before doing
// anything that costs money.
func (fixture assistantConversationServiceUnderTest) expectUsageToday(usageToday int) {
	fixture.conversationRepository.EXPECT().
		SumUsageBetween(gomock.Any(), dayStart, dayEnd).
		Return(usageToday, nil)
}

// answeredReply is the assistant answering outright.
func answeredReply(answer string, usage int) vo.AssistantReplyVo {
	return vo.AssistantReplyVo{Answer: answer, Usage: usage}
}

// queryingReply is the assistant asking for a capability to be run first.
func queryingReply(name string, usage int) vo.AssistantReplyVo {
	return vo.AssistantReplyVo{
		QueryCalls: []vo.AssistantQueryCallVo{{CallID: "call_1", Name: name, Arguments: `{}`}},
		Usage:      usage,
	}
}

func TestAskStartsAConversationWhenTheQuestionNamesNone(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(answeredReply("最近在盤整", 500), nil)

	savedConversation := fixture.expectNewConversation(42)

	startedDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, Question: "BTCUSDT 最近走勢如何"})

	require.NoError(t, askError)
	assert.Equal(t, uint(42), startedDto.ConversationID)
	assert.Equal(t, startedTurnID, startedDto.TurnID)
	assert.Equal(t, string(vo.AssistantTurnRunning), startedDto.Status)

	// It belongs to whoever asked from the moment it is written. A conversation
	// stored without an owner would be readable by everybody.
	assert.Equal(t, uint(3), savedConversation.OwnerID)
	require.Len(t, savedConversation.Turns, 1)
	assert.Equal(t, "BTCUSDT 最近走勢如何", savedConversation.Turns[0].Ask)
	assert.Equal(t, askedAt, savedConversation.LastActiveAt)

	// The question is stored before the assistant has said anything, which is what
	// makes it findable while the answer is still being written.
	assert.Equal(t, string(vo.AssistantTurnRunning), savedConversation.Turns[0].Status)
	assert.Empty(t, savedConversation.Turns[0].Answer)

	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, startedTurnID, completedTurn.ID)
	assert.Equal(t, string(vo.AssistantTurnAnswered), completedTurn.Status)
	assert.Equal(t, "最近在盤整", completedTurn.Answer)
	assert.Equal(t, 500, completedTurn.Usage)
	assert.Equal(t, 0, completedTurn.QueryCount)
	assert.False(t, completedTurn.StoppedAtQueryLimit)
}

func TestAskAddsToTheConversationTheQuestionNames(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 3, Turns: []entities.AssistantTurn{
			{Ask: "BTCUSDT 呢", Answer: "在盤整", CreatedAt: askedAt},
		}}, nil)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(answeredReply("ETHUSDT 在漲", 400), nil)
	startedTurn := fixture.expectAppendedTurn(7)

	startedDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "那 ETHUSDT 呢"})

	require.NoError(t, askError)
	assert.Equal(t, uint(7), startedDto.ConversationID)
	assert.Equal(t, "那 ETHUSDT 呢", startedTurn.Ask)
	assert.Equal(t, string(vo.AssistantTurnRunning), startedTurn.Status)

	assert.Equal(t, "ETHUSDT 在漲", fixture.awaitCompletedTurn(t).Answer)
}

func TestAskShowsTheAssistantWhatTheConversationAlreadySaid(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 3, Turns: []entities.AssistantTurn{
			{Ask: "BTCUSDT 最近走勢如何", Answer: "在盤整", CreatedAt: askedAt},
		}}, nil)

	sentRequest := vo.AssistantTurnRequestVo{}
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
			sentRequest = request

			return answeredReply("ETHUSDT 在漲", 400), nil
		})
	fixture.expectAppendedTurn(7)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "那 ETHUSDT 呢"})

	require.NoError(t, askError)
	fixture.awaitCompletedTurn(t)
	require.Len(t, sentRequest.Messages, 3)
	assert.Equal(t, "BTCUSDT 最近走勢如何", sentRequest.Messages[0].Content)
	assert.Equal(t, "在盤整", sentRequest.Messages[1].Content)
	assert.Equal(t, "那 ETHUSDT 呢", sentRequest.Messages[2].Content)
}

func TestAskTellsTheAssistantEverythingItMayDoAndNothingMore(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)

	sentRequest := vo.AssistantTurnRequestVo{}
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
			sentRequest = request

			return answeredReply("你好", 100), nil
		})
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "你好"})

	require.NoError(t, askError)
	fixture.awaitCompletedTurn(t)
	require.Len(t, sentRequest.Declarations, 1)
	assert.Equal(t, theQueryName, sentRequest.Declarations[0].Name)
	assert.Equal(t, "列出交易標的", sentRequest.Declarations[0].Description)
	assert.Equal(t, `{"type":"object"}`, sentRequest.Declarations[0].ArgumentSchema)
	assert.Equal(t, 2000, sentRequest.AnswerLengthLimit)
}

func TestAskRefusesAQuestionThatSaidNothingBeforeSpendingAnything(t *testing.T) {
	// Nothing is read and nothing is asked: the cheapest refusal must not pay for the
	// more expensive one's lookup.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "   "})

	require.ErrorIs(t, askError, domains.ErrAssistantAskEmpty)
}

func TestAskReportsAConversationThatIsNotThere(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Conversation{}, domains.ConversationNotFound(99))

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 99, Question: "BTCUSDT 最近走勢如何"})

	require.ErrorIs(t, askError, domains.ErrConversationNotFound)
}

func TestAskRefusesOnceTodaysAllowanceIsSpent(t *testing.T) {
	testCases := []struct {
		name       string
		usageToday int
	}{
		{name: "reaching the ceiling exactly", usageToday: 300000},
		{name: "past the ceiling", usageToday: 400000},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(testCase.usageToday)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

			require.ErrorIs(t, askError, domains.ErrDailyUsageAllowanceExhausted)
			assert.Contains(t, askError.Error(), "2026-09-05T00:00:00Z")
		})
	}
}

func TestAskAnswersInFullWhenTheAllowanceIsOnlySpentAfterwards(t *testing.T) {
	// The allowance is settled before the answer, because what an answer costs is
	// only known once it exists. Overshooting by one exchange is the accepted price of
	// never refusing an answer that was within the ceiling when it started.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(299999)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(answeredReply("最近在盤整", 5000), nil)
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.NoError(t, askError)
	assert.Equal(t, 5000, fixture.awaitCompletedTurn(t).Usage)
}

func TestAskRunsTheCapabilityTheAssistantAskedFor(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), `{}`).Return(`{"symbols":["BTCUSDT"]}`, nil)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
				// What it looked at comes back to it on the next round trip, which is
				// how it knows what it has already learned.
				require.Len(t, request.Rounds, 1)
				require.Len(t, request.Rounds[0].Exchanges, 1)
				assert.Equal(t, `{"symbols":["BTCUSDT"]}`, request.Rounds[0].Exchanges[0].Outcome)
				assert.False(t, request.Rounds[0].Exchanges[0].Rejected)

				return answeredReply("有 BTCUSDT", 200), nil
			}),
	)

	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "有哪些交易標的"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, "有 BTCUSDT", completedTurn.Answer)
	assert.Equal(t, 1, completedTurn.QueryCount)
	// Every round trip is paid for, not just the one that answered.
	assert.Equal(t, 300, completedTurn.Usage)
	require.Len(t, completedTurn.Queries, 1)
	assert.Equal(t, theQueryName, completedTurn.Queries[0].QueryName)
}

func TestAskDoesNotMistakeWhatTheAssistantSaysOnTheWayForAnAnswer(t *testing.T) {
	// 這是回報進來的症狀：問「給我一份布林通道的腳本」，回來的是
	// 「我先看一下系統裡既有策略腳本的算式寫法」然後就結束了，工具一次都沒跑，
	// 使用者只好自己再問一次「好了沒」。
	//
	// 助手很常在**同一則回覆裡**同時說一句話與要求一次查詢。那句話是旁白不是答案，
	// 所以要先問「有沒有要查」再問「有沒有說話」。
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(`{"strategyScripts":[]}`, nil)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(vo.AssistantReplyVo{
				Answer:     "我先看一下系統裡既有策略腳本的算式寫法。",
				QueryCalls: []vo.AssistantQueryCallVo{{CallID: "call_1", Name: theQueryName, Arguments: `{}`}},
				Usage:      100,
			}, nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
				// 那句旁白要跟著它的查詢請求一起回去，否則助手是從一個它看不到的
				// 想法往下接，答案會從半句話開始。
				require.Len(t, request.Rounds, 1)
				assert.Equal(t, "我先看一下系統裡既有策略腳本的算式寫法。", request.Rounds[0].Narration)

				return answeredReply("這是一份布林通道的算式：…", 200), nil
			}),
	)

	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "請給我一份布林通道的腳本"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, "這是一份布林通道的算式：…", completedTurn.Answer)
	assert.NotContains(t, completedTurn.Answer, "我先看一下")
	assert.Equal(t, 1, completedTurn.QueryCount)
	assert.Len(t, completedTurn.Queries, 1)
}

func TestAskAnswersWithWhatItSaidWhenItsQueriesAreSpent(t *testing.T) {
	// 查詢次數用完之後，助手又只給了一句話與一個要不到的查詢請求。
	// 那句話就是它手上僅有的東西——連同「已達上限」的標記一起留下，
	// 比回一句「助手沒有回應」誠實。
	fixture := newAssistantConversationServiceUnderTest(t, 1, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return("{}", nil)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(vo.AssistantReplyVo{
				Answer:     "只查到這些，還缺歷史資料。",
				QueryCalls: []vo.AssistantQueryCallVo{{CallID: "call_2", Name: theQueryName, Arguments: `{}`}},
				Usage:      100,
			}, nil),
	)
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查到底"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, "只查到這些，還缺歷史資料。", completedTurn.Answer)
	assert.True(t, completedTurn.StoppedAtQueryLimit)
}

func TestAskRunsEveryCapabilityAskedForAtOnce(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return("{}", nil).Times(2)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(vo.AssistantReplyVo{
				QueryCalls: []vo.AssistantQueryCallVo{
					{CallID: "call_1", Name: theQueryName, Arguments: `{}`},
					{CallID: "call_2", Name: theQueryName, Arguments: `{}`},
				},
				Usage: 100,
			}, nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(answeredReply("查完了", 100), nil),
	)
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查兩次"})

	require.NoError(t, askError)
	assert.Equal(t, 2, fixture.awaitCompletedTurn(t).QueryCount)
}

func TestAskHandsARefusalBackToTheAssistantInsteadOfGivingUp(t *testing.T) {
	testCases := []struct {
		name            string
		requestedName   string
		runOutcome      string
		runError        error
		expectedOutcome string
		expectsRun      bool
	}{
		{
			name:          "a capability that refused the arguments",
			requestedName: theQueryName,
			runError:      errors.New("彙總刻度只接受 5m、15m、1h、4h、1d"),
			// The assistant reads the reason and may ask differently; ending the
			// answer here would throw away every lookup that already worked.
			expectedOutcome: "彙總刻度只接受 5m、15m、1h、4h、1d",
			expectsRun:      true,
		},
		{
			name:            "a capability that does not exist at all",
			requestedName:   "delete_strategyScript",
			expectedOutcome: "系統沒有「delete_strategyScript」這個能力",
			expectsRun:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			if testCase.expectsRun {
				fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(testCase.runOutcome, testCase.runError)
			}

			gomock.InOrder(
				fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
					Return(queryingReply(testCase.requestedName, 100), nil),
				fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
						require.Len(t, request.Rounds, 1)
						require.Len(t, request.Rounds[0].Exchanges, 1)
						assert.Contains(t, request.Rounds[0].Exchanges[0].Outcome, testCase.expectedOutcome)
						assert.True(t, request.Rounds[0].Exchanges[0].Rejected)

						return answeredReply("這件事辦不到", 100), nil
					}),
			)

			fixture.expectNewConversation(1)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{Question: "幫我做那件事"})

			require.NoError(t, askError)
			completedTurn := fixture.awaitCompletedTurn(t)
			assert.Equal(t, "這件事辦不到", completedTurn.Answer)
			require.Len(t, completedTurn.Queries, 1)
			assert.True(t, completedTurn.Queries[0].Rejected)
		})
	}
}

func TestAskStopsRunningCapabilitiesOnceTheirLimitIsSpent(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 2, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return("{}", nil).Times(2)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
				// Being told is what turns a half answer into an honest one.
				assert.True(t, request.QueryLimitReached)

				return answeredReply("只查到這些", 100), nil
			}),
	)

	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查到底"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, "只查到這些", completedTurn.Answer)
	assert.Equal(t, 2, completedTurn.QueryCount)
	assert.True(t, completedTurn.StoppedAtQueryLimit)
}

func TestAskStopsPartWayThroughARoundThatWouldOverspend(t *testing.T) {
	// Asking for three lookups at once does not buy three when only one is left. The
	// count is spent per lookup, so the round is cut short rather than let through.
	fixture := newAssistantConversationServiceUnderTest(t, 1, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return("{}", nil).Times(1)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(vo.AssistantReplyVo{
				QueryCalls: []vo.AssistantQueryCallVo{
					{CallID: "call_1", Name: theQueryName, Arguments: `{}`},
					{CallID: "call_2", Name: theQueryName, Arguments: `{}`},
					{CallID: "call_3", Name: theQueryName, Arguments: `{}`},
				},
				Usage: 100,
			}, nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(answeredReply("只查到一次", 100), nil),
	)
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "一次查三個"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, 1, completedTurn.QueryCount)
	assert.True(t, completedTurn.StoppedAtQueryLimit)
}

func TestAskRecordsAFailureWhenTheAssistantAsksForMoreItCannotHave(t *testing.T) {
	// Its queries are spent and it was told so, and it still asked instead of
	// speaking. There is no answer to write, so the exchange is closed as failed —
	// which is what the asker sees when they come back to it.
	fixture := newAssistantConversationServiceUnderTest(t, 1, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return("{}", nil)
	fixture.expectNewConversation(1)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
	)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查到底"})

	require.NoError(t, askError)
	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, string(vo.AssistantTurnFailed), completedTurn.Status)
	assert.NotEmpty(t, completedTurn.FailureReason)
}

func TestAskRecordsAFailureWhenTheAssistantDoesNotAnswer(t *testing.T) {
	testCases := []struct {
		name  string
		reply vo.AssistantReplyVo
		err   error
	}{
		{name: "unreachable", err: errors.New("dial tcp: connection refused")},
		{name: "too slow", err: errors.New("context deadline exceeded")},
		{name: "said nothing at all", reply: vo.AssistantReplyVo{Usage: 100}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// It used to leave nothing behind, and that was right while somebody was
			// watching the screen: send, fail, see the error, retype, all within
			// seconds. An answer that takes minutes breaks that — the asker is not
			// there — so a row saying it failed is the only way they can tell it
			// apart from one still running and one they never sent.
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(entities.Conversation{ID: 7, OwnerID: 3}, nil)
			fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
				Return(testCase.reply, testCase.err)
			fixture.expectAppendedTurn(7)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "BTCUSDT 最近走勢如何"})

			// Accepting the question succeeded; it is the answer that failed, and
			// that is a fact about the exchange rather than about the ask.
			require.NoError(t, askError)

			completedTurn := fixture.awaitCompletedTurn(t)
			assert.Equal(t, startedTurnID, completedTurn.ID)
			assert.Equal(t, string(vo.AssistantTurnFailed), completedTurn.Status)
			assert.Contains(t, completedTurn.FailureReason, "請稍後再試")
			// Nobody is charged for an answer they never got.
			assert.Equal(t, 0, completedTurn.Usage)
		})
	}
}

func TestAskRefusesASecondQuestionWhileTheFirstAnswerIsStillBeingWritten(t *testing.T) {
	// Two answers written into one conversation at once leaves nobody able to say
	// which of them the record belongs to. The moment somebody would do it is almost
	// always the one this whole design removes: believing the first never sent.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 3, Turns: []entities.AssistantTurn{
			{ID: 1, Ask: "前一句", Status: string(vo.AssistantTurnRunning), CreatedAt: askedAt},
		}}, nil)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "再問一句"})

	require.ErrorIs(t, askError, domains.ErrAssistantAnswerInProgress)
}

func TestAskAcceptsTheNextQuestionOnceTheExchangeBeforeItHasEnded(t *testing.T) {
	testCases := []struct {
		name          string
		previousState string
	}{
		{name: "the one before it was answered", previousState: string(vo.AssistantTurnAnswered)},
		// A failed exchange is not in flight. Nothing is still being written, so
		// there is nothing a second question could collide with — and making somebody
		// wait on a failure would leave the conversation permanently unusable.
		{name: "the one before it failed", previousState: string(vo.AssistantTurnFailed)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(entities.Conversation{ID: 7, OwnerID: 3, Turns: []entities.AssistantTurn{
					{ID: 1, Ask: "前一句", Answer: "上次的答案",
						Status: testCase.previousState, CreatedAt: askedAt},
				}}, nil)
			fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
				Return(answeredReply("這次的答案", 100), nil)
			fixture.expectAppendedTurn(7)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "再問一句"})

			require.NoError(t, askError)
			assert.Equal(t, "這次的答案", fixture.awaitCompletedTurn(t).Answer)
		})
	}
}

func TestAskDoesNotShowTheAssistantAQuestionThatWasNeverAnswered(t *testing.T) {
	// A question with nothing under it reads to the assistant as one it declined to
	// answer, and it will go on to explain why it declined — which is not what
	// happened.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 3, Turns: []entities.AssistantTurn{
			{ID: 1, Ask: "答得出來的那句", Answer: "答案", CreatedAt: askedAt,
				Status: string(vo.AssistantTurnAnswered)},
			{ID: 2, Ask: "壞掉的那句", CreatedAt: askedAt,
				Status: string(vo.AssistantTurnFailed), FailureReason: "助手沒有回應"},
		}}, nil)

	sentRequest := vo.AssistantTurnRequestVo{}
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
			sentRequest = request

			return answeredReply("好的", 100), nil
		})
	fixture.expectAppendedTurn(7)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "再問一句"})

	require.NoError(t, askError)
	fixture.awaitCompletedTurn(t)

	require.Len(t, sentRequest.Messages, 3)
	assert.Equal(t, "答得出來的那句", sentRequest.Messages[0].Content)
	assert.Equal(t, "答案", sentRequest.Messages[1].Content)
	assert.Equal(t, "再問一句", sentRequest.Messages[2].Content)
}

func TestFailInterruptedAnswersClearsWhatTheLastShutdownCutOff(t *testing.T) {
	// An answer being written lives in this process and nowhere else, so every one
	// left at running is stale the moment this one starts. Left alone each is a wait
	// nobody can end, on a conversation nobody can add to.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)

	sweptReason := ""
	fixture.conversationRepository.EXPECT().FailAllRunningTurns(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reason string) (int, error) {
			sweptReason = reason

			return 3, nil
		})

	interruptedCount, sweepError := fixture.assistantConversationService.FailInterruptedAnswers(
		t.Context())

	require.NoError(t, sweepError)
	assert.Equal(t, 3, interruptedCount)
	assert.Contains(t, sweptReason, "重新啟動")
}

func TestFailInterruptedAnswersReportsAFailureToSweep(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FailAllRunningTurns(gomock.Any(), gomock.Any()).
		Return(0, errors.New("storage unavailable"))

	_, sweepError := fixture.assistantConversationService.FailInterruptedAnswers(t.Context())

	require.Error(t, sweepError)
}

func TestAskReportsAFailureToReadTodaysUsage(t *testing.T) {
	// The allowance cannot be honoured without it, and answering anyway would make the
	// one ceiling that makes the bill impossible optional.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().SumUsageBetween(gomock.Any(), dayStart, dayEnd).
		Return(0, errors.New("storage unavailable"))

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.Error(t, askError)
	assert.Contains(t, askError.Error(), "storage unavailable")
}

func TestAskReportsAFailureToReserveThePlaceTheAnswerWouldGo(t *testing.T) {
	// The assistant is never asked. Reserving the place is what the asker is waiting
	// on, so failing it is a failure of the ask itself rather than of an answer.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).Times(0)
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{}, errors.New("storage unavailable"))

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.Error(t, askError)
	assert.Contains(t, askError.Error(), "storage unavailable")
}

func TestListConversationsPutsTheMostRecentlyActiveFirst(t *testing.T) {
	// The store is what orders them; this proves the order survives being turned into
	// what a reader sees, and that the message count comes along.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindAllOwnedBy(gomock.Any(), uint(3)).
		Return([]entities.Conversation{
			{ID: 2, LastActiveAt: askedAt, Turns: []entities.AssistantTurn{
				{Ask: "問", Answer: "答", CreatedAt: askedAt},
			}},
			{ID: 1, LastActiveAt: askedAt.Add(-time.Hour)},
		}, nil)

	summaryDtos, listError := fixture.assistantConversationService.ListConversations(t.Context(), 3)

	require.NoError(t, listError)
	require.Len(t, summaryDtos, 2)
	assert.Equal(t, uint(2), summaryDtos[0].ID)
	assert.Equal(t, 2, summaryDtos[0].MessageCount)
	assert.Equal(t, uint(1), summaryDtos[1].ID)
	assert.Equal(t, 0, summaryDtos[1].MessageCount)
}

func TestListConversationsAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindAllOwnedBy(gomock.Any(), uint(3)).
		Return([]entities.Conversation{}, nil)

	summaryDtos, listError := fixture.assistantConversationService.ListConversations(t.Context(), 3)

	require.NoError(t, listError)
	assert.Empty(t, summaryDtos)
}

func TestListConversationsReportsAFailureToRead(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindAllOwnedBy(gomock.Any(), uint(3)).
		Return(nil, errors.New("storage unavailable"))

	_, listError := fixture.assistantConversationService.ListConversations(t.Context(), 3)

	require.Error(t, listError)
}

func TestGetConversationHandsBackEveryMessageEverSaid(t *testing.T) {
	// Neither today's allowance nor the assistant is consulted: the brake is on new
	// answers, and an assistant that is down must not take the record with it.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 3, LastActiveAt: askedAt, Turns: []entities.AssistantTurn{
			{Ask: "問 1", Answer: "答 1", CreatedAt: askedAt},
			{Ask: "問 2", Answer: "答 2", CreatedAt: askedAt},
		}}, nil)

	conversationDto, findError := fixture.assistantConversationService.GetConversation(t.Context(), 3, 7)

	require.NoError(t, findError)
	require.Len(t, conversationDto.Messages, 4)
	assert.Equal(t, "問 1", conversationDto.Messages[0].Content)
	assert.Equal(t, "答 2", conversationDto.Messages[3].Content)
}

func TestGetConversationRefusesSomebodyElsesAsOneThatIsNotThere(t *testing.T) {
	// A transcript is not only what was said: the assistant acts as whoever asked
	// it, so an exchange can hold that person's own algorithms in full. Told apart
	// from one that does not exist, this refusal would also say whose it is.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 9, LastActiveAt: askedAt}, nil)

	_, findError := fixture.assistantConversationService.GetConversation(t.Context(), 3, 7)

	require.ErrorIs(t, findError, domains.ErrConversationNotFound)
}

func TestAskRefusesSomebodyElsesConversationBeforeAskingTheAssistant(t *testing.T) {
	// Refused at the first read of that conversation, so the assistant is never
	// shown a word of it — nor paid for.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 9, Turns: []entities.AssistantTurn{
			{Ask: "別人問的", Answer: "別人的答案", CreatedAt: askedAt},
		}}, nil)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "接著說"})

	require.ErrorIs(t, askError, domains.ErrConversationNotFound)
}

func TestGetConversationReportsOneThatIsNotThere(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Conversation{}, domains.ConversationNotFound(99))

	_, findError := fixture.assistantConversationService.GetConversation(t.Context(), 3, 99)

	require.ErrorIs(t, findError, domains.ErrConversationNotFound)
}
