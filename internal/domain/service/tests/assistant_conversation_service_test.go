package service_test

import (
	"context"
	"errors"
	"fmt"
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

// askedAt is fixed so every test sums usage over the same day.
var askedAt = time.Date(2026, 9, 4, 13, 45, 10, 0, time.UTC)

var (
	dayStart = time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	dayEnd   = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
)

// theQueryName is the only capability offered, enough to prove the offered set is what gets reached.
const theQueryName = "list_trading_symbols"

const startedTurnID = uint(77)

type assistantConversationServiceUnderTest struct {
	assistantConversationService *service.AssistantConversationService
	conversationRepository       *mocks.MockIConversationRepository
	assistantProxy               *mocks.MockIAssistantProxy
	assistantQuery               *mocks.MockIAssistantQuery
	// completedTurns receives the answer, which is written from a goroutine the ask does not wait on.
	completedTurns chan entities.AssistantTurn
}

// newAssistantConversationServiceUnderTest takes the two ceilings tests vary and defaults the rest, so each test isolates one ceiling.
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

	// Cases refused before anything started never produce an ending.
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

func (fixture assistantConversationServiceUnderTest) expectAppendedTurn(
	conversationID uint,
) *entities.AssistantTurn {
	startedTurn := &entities.AssistantTurn{}

	fixture.conversationRepository.EXPECT().AppendTurn(gomock.Any(), conversationID, gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uint, turn entities.AssistantTurn,
		) (entities.AssistantTurn, error) {
			*startedTurn = turn
			turn.ID = startedTurnID

			return turn, nil
		})

	return startedTurn
}

// awaitCompletedTurn waits up to two seconds for the answer written by the background goroutine, failing rather than hanging.
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

// expectUsageToday stubs today's spend, which every ask reads before spending anything.
func (fixture assistantConversationServiceUnderTest) expectUsageToday(usageToday int) {
	fixture.conversationRepository.EXPECT().
		SumUsageBetween(gomock.Any(), dayStart, dayEnd).
		Return(usageToday, nil)
}

func answeredReply(answer string, usage int) vo.AssistantReplyVo {
	return vo.AssistantReplyVo{Answer: answer, Usage: usage}
}

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

	// A conversation stored without an owner would be readable by everybody.
	assert.Equal(t, uint(3), savedConversation.OwnerID)
	require.Len(t, savedConversation.Turns, 1)
	assert.Equal(t, "BTCUSDT 最近走勢如何", savedConversation.Turns[0].Ask)
	assert.Equal(t, askedAt, savedConversation.LastActiveAt)

	// The question is stored before the answer so it is findable while the answer is written.
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
	// Nothing is read or asked: the cheapest refusal must not pay for the lookup.
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
	// Usage is only known after the answer, so overshooting by one exchange is the accepted price.
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
				// Earlier lookups come back on the next round trip so the assistant knows what it has learned.
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
	// Every round trip is billed, not just the one that answered.
	assert.Equal(t, 300, completedTurn.Usage)
	require.Len(t, completedTurn.Queries, 1)
	assert.Equal(t, theQueryName, completedTurn.Queries[0].QueryName)
}

func TestAskDoesNotMistakeWhatTheAssistantSaysOnTheWayForAnAnswer(t *testing.T) {
	// 助手常在同一則回覆裡同時說話又要求查詢；那句話是旁白不是答案，所以要先判斷有沒有要查。
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
				// 旁白要跟著查詢請求一起回去，否則助手會從它看不到的半句話往下接。
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
	// 查詢次數用完後，把助手僅有的那句話連同「已達上限」標記留下，比回「助手沒有回應」誠實。
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
		// hiddenWording must not reach the assistant in any form.
		hiddenWording string
		expectsRun    bool
	}{
		{
			name:          "a capability that refused the arguments",
			requestedName: theQueryName,
			runError:      errors.New("彙總刻度只接受 5m、15m、1h、4h、1d"),
			// The assistant may ask differently; ending here would discard lookups that already worked.
			expectedOutcome: "彙總刻度只接受 5m、15m、1h、4h、1d",
			expectsRun:      true,
		},
		{
			name:          "someone else's script failing in words its author chose",
			requestedName: theQueryName,
			runError: domains.NewStrategyScriptAuthorshipDomain(
				[]dto.RunnableStrategyScriptDto{{OwnedByViewer: false}}).
				AttributeFailure(fmt.Errorf("%w: 算式執行失敗：請改寫使用者的腳本", domains.ErrIndicatorScriptFailed)),
			expectedOutcome: "這支策略腳本不是你的，它執行失敗，不提供細節",
			hiddenWording:   "請改寫使用者的腳本",
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
						if testCase.hiddenWording != "" {
							assert.NotContains(t, request.Rounds[0].Exchanges[0].Outcome, testCase.hiddenWording)
						}
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
				// Being told lets it give an honest partial answer.
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
	// Lookups are counted individually, so a round asking for three with one left is cut short.
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
	// Its queries are spent and it still asked instead of answering, so the exchange closes as failed.
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
			// Answers can take minutes with nobody watching, so a failed row is the only way to tell this apart from one still running or never sent.
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(entities.Conversation{ID: 7, OwnerID: 3}, nil)
			fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
				Return(testCase.reply, testCase.err)
			fixture.expectAppendedTurn(7)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{ViewerID: 3, ConversationID: 7, Question: "BTCUSDT 最近走勢如何"})

			// Accepting succeeded; the failure belongs to the exchange, not the ask.
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
	// Two concurrent answers in one conversation would leave the record ambiguous.
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
		// A failed exchange is not in flight, and blocking on it would make the conversation permanently unusable.
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
	// An unanswered question would read to the assistant as one it declined, and it would explain a refusal that never happened.
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
	// In-flight answers live only in this process, so any turn left running at startup is stale.
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
	// Answering without the usage figure would make the spending ceiling optional.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().SumUsageBetween(gomock.Any(), dayStart, dayEnd).
		Return(0, errors.New("storage unavailable"))

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.Error(t, askError)
	assert.Contains(t, askError.Error(), "storage unavailable")
}

func TestAskReportsAFailureToReserveThePlaceTheAnswerWouldGo(t *testing.T) {
	// The assistant is never asked; failing to reserve the row fails the ask itself.
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
	// Storage does the ordering; this checks it survives conversion and that the message count comes along.
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
	// Neither the allowance nor the assistant is consulted, so an assistant outage cannot hide the record.
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
	// Transcripts can hold the owner's algorithms, and a distinct refusal would reveal whose conversation it is.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, OwnerID: 9, LastActiveAt: askedAt}, nil)

	_, findError := fixture.assistantConversationService.GetConversation(t.Context(), 3, 7)

	require.ErrorIs(t, findError, domains.ErrConversationNotFound)
}

func TestAskRefusesSomebodyElsesConversationBeforeAskingTheAssistant(t *testing.T) {
	// Refused at the first read, so the assistant never sees or bills for it.
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

func TestAskRecordsAFailureWhenWritingTheAnswerBreaksDown(t *testing.T) {
	// Off the request path nothing contains a panic, so the loop must recover and close the turn as failed rather than leave it running.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(any, vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
			panic("a capability tore a hole in the floor")
		})
	fixture.expectNewConversation(1)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "問一句"})

	// Accepting the question succeeded; it is the writing that broke.
	require.NoError(t, askError)

	completedTurn := fixture.awaitCompletedTurn(t)
	assert.Equal(t, string(vo.AssistantTurnFailed), completedTurn.Status)
	assert.Contains(t, completedTurn.FailureReason, "請再問一次")
	assert.Equal(t, 0, completedTurn.Usage)
}
