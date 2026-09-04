package service_test

import (
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

type assistantConversationServiceUnderTest struct {
	assistantConversationService *service.AssistantConversationService
	conversationRepository       *mocks.MockIConversationRepository
	assistantProxy               *mocks.MockIAssistantProxy
	assistantQuery               *mocks.MockIAssistantQuery
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

	savedConversation := entities.Conversation{}
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, conversation entities.Conversation) (entities.Conversation, error) {
			savedConversation = conversation
			conversation.ID = 42

			return conversation, nil
		})

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.NoError(t, askError)
	assert.Equal(t, uint(42), answerDto.ConversationID)
	assert.Equal(t, "最近在盤整", answerDto.Answer)
	assert.Equal(t, 500, answerDto.Usage)
	assert.Equal(t, 0, answerDto.QueryCount)
	assert.False(t, answerDto.StoppedAtQueryLimit)

	require.Len(t, savedConversation.Turns, 1)
	assert.Equal(t, "BTCUSDT 最近走勢如何", savedConversation.Turns[0].Ask)
	assert.Equal(t, "最近在盤整", savedConversation.Turns[0].Answer)
	assert.Equal(t, askedAt, savedConversation.LastActiveAt)
}

func TestAskAddsToTheConversationTheQuestionNames(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, Turns: []entities.AssistantTurn{
			{Ask: "BTCUSDT 呢", Answer: "在盤整", CreatedAt: askedAt},
		}}, nil)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(answeredReply("ETHUSDT 在漲", 400), nil)
	fixture.conversationRepository.EXPECT().AppendTurn(gomock.Any(), uint(7), gomock.Any()).
		Return(entities.Conversation{ID: 7}, nil)

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ConversationID: 7, Question: "那 ETHUSDT 呢"})

	require.NoError(t, askError)
	assert.Equal(t, uint(7), answerDto.ConversationID)
	assert.Equal(t, "ETHUSDT 在漲", answerDto.Answer)
}

func TestAskShowsTheAssistantWhatTheConversationAlreadySaid(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, Turns: []entities.AssistantTurn{
			{Ask: "BTCUSDT 最近走勢如何", Answer: "在盤整", CreatedAt: askedAt},
		}}, nil)

	sentRequest := vo.AssistantTurnRequestVo{}
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
			sentRequest = request

			return answeredReply("ETHUSDT 在漲", 400), nil
		})
	fixture.conversationRepository.EXPECT().AppendTurn(gomock.Any(), uint(7), gomock.Any()).
		Return(entities.Conversation{ID: 7}, nil)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ConversationID: 7, Question: "那 ETHUSDT 呢"})

	require.NoError(t, askError)
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
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{ID: 1}, nil)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "你好"})

	require.NoError(t, askError)
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
		t.Context(), dto.AssistantAskDto{ConversationID: 7, Question: "   "})

	require.ErrorIs(t, askError, domains.ErrAssistantAskEmpty)
}

func TestAskReportsAConversationThatIsNotThere(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Conversation{}, domains.ConversationNotFound(99))

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{ConversationID: 99, Question: "BTCUSDT 最近走勢如何"})

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
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{ID: 1}, nil)

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "BTCUSDT 最近走勢如何"})

	require.NoError(t, askError)
	assert.Equal(t, 5000, answerDto.Usage)
}

func TestAskRunsTheCapabilityTheAssistantAskedFor(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), `{}`).Return(`{"symbols":["BTCUSDT"]}`, nil)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
				// What it looked at comes back to it on the next round trip, which is
				// how it knows what it has already learned.
				require.Len(t, request.Exchanges, 1)
				assert.Equal(t, `{"symbols":["BTCUSDT"]}`, request.Exchanges[0].Outcome)
				assert.False(t, request.Exchanges[0].Rejected)

				return answeredReply("有 BTCUSDT", 200), nil
			}),
	)

	storedTurn := entities.AssistantTurn{}
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, conversation entities.Conversation) (entities.Conversation, error) {
			storedTurn = conversation.Turns[0]
			conversation.ID = 1

			return conversation, nil
		})

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "有哪些交易標的"})

	require.NoError(t, askError)
	assert.Equal(t, "有 BTCUSDT", answerDto.Answer)
	assert.Equal(t, 1, answerDto.QueryCount)
	// Every round trip is paid for, not just the one that answered.
	assert.Equal(t, 300, answerDto.Usage)
	require.Len(t, storedTurn.Queries, 1)
	assert.Equal(t, theQueryName, storedTurn.Queries[0].QueryName)
}

func TestAskRunsEveryCapabilityAskedForAtOnce(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any()).Return("{}", nil).Times(2)

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
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{ID: 1}, nil)

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查兩次"})

	require.NoError(t, askError)
	assert.Equal(t, 2, answerDto.QueryCount)
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
			requestedName:   "delete_strategy",
			expectedOutcome: "系統沒有「delete_strategy」這個能力",
			expectsRun:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			if testCase.expectsRun {
				fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any()).
					Return(testCase.runOutcome, testCase.runError)
			}

			gomock.InOrder(
				fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
					Return(queryingReply(testCase.requestedName, 100), nil),
				fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ any, request vo.AssistantTurnRequestVo) (vo.AssistantReplyVo, error) {
						require.Len(t, request.Exchanges, 1)
						assert.Contains(t, request.Exchanges[0].Outcome, testCase.expectedOutcome)
						assert.True(t, request.Exchanges[0].Rejected)

						return answeredReply("這件事辦不到", 100), nil
					}),
			)

			storedTurn := entities.AssistantTurn{}
			fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ any, conversation entities.Conversation) (entities.Conversation, error) {
					storedTurn = conversation.Turns[0]
					conversation.ID = 1

					return conversation, nil
				})

			answerDto, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{Question: "幫我做那件事"})

			require.NoError(t, askError)
			assert.Equal(t, "這件事辦不到", answerDto.Answer)
			require.Len(t, storedTurn.Queries, 1)
			assert.True(t, storedTurn.Queries[0].Rejected)
		})
	}
}

func TestAskStopsRunningCapabilitiesOnceTheirLimitIsSpent(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 2, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any()).Return("{}", nil).Times(2)

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

	storedTurn := entities.AssistantTurn{}
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, conversation entities.Conversation) (entities.Conversation, error) {
			storedTurn = conversation.Turns[0]
			conversation.ID = 1

			return conversation, nil
		})

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查到底"})

	require.NoError(t, askError)
	assert.Equal(t, "只查到這些", answerDto.Answer)
	assert.Equal(t, 2, answerDto.QueryCount)
	assert.True(t, answerDto.StoppedAtQueryLimit)
	assert.True(t, storedTurn.StoppedAtQueryLimit)
}

func TestAskStopsPartWayThroughARoundThatWouldOverspend(t *testing.T) {
	// Asking for three lookups at once does not buy three when only one is left. The
	// count is spent per lookup, so the round is cut short rather than let through.
	fixture := newAssistantConversationServiceUnderTest(t, 1, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any()).Return("{}", nil).Times(1)

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
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{ID: 1}, nil)

	answerDto, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "一次查三個"})

	require.NoError(t, askError)
	assert.Equal(t, 1, answerDto.QueryCount)
	assert.True(t, answerDto.StoppedAtQueryLimit)
}

func TestAskGivesUpWhenTheAssistantAsksForMoreItCannotHave(t *testing.T) {
	// Its queries are spent and it was told so, and it still asked instead of
	// speaking. There is nothing left to run and no answer to store, so this is the
	// same nothing as an assistant that never answered.
	fixture := newAssistantConversationServiceUnderTest(t, 1, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantQuery.EXPECT().Run(gomock.Any(), gomock.Any()).Return("{}", nil)

	gomock.InOrder(
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
		fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
			Return(queryingReply(theQueryName, 100), nil),
	)

	_, askError := fixture.assistantConversationService.Ask(
		t.Context(), dto.AssistantAskDto{Question: "查到底"})

	require.ErrorIs(t, askError, domains.ErrAssistantUnavailable)
}

func TestAskLeavesNothingBehindWhenTheAssistantDoesNotAnswer(t *testing.T) {
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
			// Nothing is stored, so a question with no answer under it never enters
			// the record — and the conversation it was aimed at is untouched.
			fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
			fixture.expectUsageToday(0)
			fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(entities.Conversation{ID: 7}, nil)
			fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
				Return(testCase.reply, testCase.err)

			_, askError := fixture.assistantConversationService.Ask(
				t.Context(), dto.AssistantAskDto{ConversationID: 7, Question: "BTCUSDT 最近走勢如何"})

			require.ErrorIs(t, askError, domains.ErrAssistantUnavailable)
		})
	}
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

func TestAskReportsAFailureToStoreTheExchange(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.expectUsageToday(0)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(answeredReply("最近在盤整", 500), nil)
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
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.Conversation{
			{ID: 2, LastActiveAt: askedAt, Turns: []entities.AssistantTurn{
				{Ask: "問", Answer: "答", CreatedAt: askedAt},
			}},
			{ID: 1, LastActiveAt: askedAt.Add(-time.Hour)},
		}, nil)

	summaryDtos, listError := fixture.assistantConversationService.ListConversations(t.Context())

	require.NoError(t, listError)
	require.Len(t, summaryDtos, 2)
	assert.Equal(t, uint(2), summaryDtos[0].ID)
	assert.Equal(t, 2, summaryDtos[0].MessageCount)
	assert.Equal(t, uint(1), summaryDtos[1].ID)
	assert.Equal(t, 0, summaryDtos[1].MessageCount)
}

func TestListConversationsAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.Conversation{}, nil)

	summaryDtos, listError := fixture.assistantConversationService.ListConversations(t.Context())

	require.NoError(t, listError)
	assert.Empty(t, summaryDtos)
}

func TestListConversationsReportsAFailureToRead(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return(nil, errors.New("storage unavailable"))

	_, listError := fixture.assistantConversationService.ListConversations(t.Context())

	require.Error(t, listError)
}

func TestGetConversationHandsBackEveryMessageEverSaid(t *testing.T) {
	// Neither today's allowance nor the assistant is consulted: the brake is on new
	// answers, and an assistant that is down must not take the record with it.
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, LastActiveAt: askedAt, Turns: []entities.AssistantTurn{
			{Ask: "問 1", Answer: "答 1", CreatedAt: askedAt},
			{Ask: "問 2", Answer: "答 2", CreatedAt: askedAt},
		}}, nil)

	conversationDto, findError := fixture.assistantConversationService.GetConversation(t.Context(), 7)

	require.NoError(t, findError)
	require.Len(t, conversationDto.Messages, 4)
	assert.Equal(t, "問 1", conversationDto.Messages[0].Content)
	assert.Equal(t, "答 2", conversationDto.Messages[3].Content)
}

func TestGetConversationReportsOneThatIsNotThere(t *testing.T) {
	fixture := newAssistantConversationServiceUnderTest(t, 8, 300000)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Conversation{}, domains.ConversationNotFound(99))

	_, findError := fixture.assistantConversationService.GetConversation(t.Context(), 99)

	require.ErrorIs(t, findError, domains.ErrConversationNotFound)
}
