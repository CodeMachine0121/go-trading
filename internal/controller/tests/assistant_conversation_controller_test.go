package controller_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var chatAskedAt = time.Date(2026, 9, 4, 13, 45, 10, 0, time.UTC)

type chatRouterUnderTest struct {
	engine                 *gin.Engine
	conversationRepository *mocks.MockIConversationRepository
	assistantProxy         *mocks.MockIAssistantProxy
}

// newChatRouterUnderTest wires the real service and real domain models, mocking only
// the outermost boundaries: storage, the assistant and the clock. What is under test
// here is which status code each refusal comes out as.
func newChatRouterUnderTest(t *testing.T) chatRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	conversationRepository := mocks.NewMockIConversationRepository(mockController)
	assistantProxy := mocks.NewMockIAssistantProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(chatAskedAt).AnyTimes()

	assistantConversationController := controller.NewAssistantConversationController(
		application.NewAssistantConversationApplication(
			service.NewAssistantConversationService(
				conversationRepository,
				assistantProxy,
				[]domaininterface.IAssistantQuery{},
				clockProxy,
				20, 8, 300000, 2000,
			)))

	engine := gin.New()
	engine.POST("/chat", assistantConversationController.Ask)
	engine.GET("/chat/conversations", assistantConversationController.ListConversations)
	engine.GET("/chat/conversations/:id", assistantConversationController.GetConversation)

	return chatRouterUnderTest{
		engine:                 engine,
		conversationRepository: conversationRepository,
		assistantProxy:         assistantProxy,
	}
}

func (fixture chatRouterUnderTest) send(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// expectUsageToday says what has been spent today, which every ask reads first.
func (fixture chatRouterUnderTest) expectUsageToday(usageToday int) {
	fixture.conversationRepository.EXPECT().
		SumUsageBetween(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(usageToday, nil)
}

func TestChatAskAnswersWithTheConversationItLandedIn(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.expectUsageToday(0)
	fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
		Return(vo.AssistantReplyVo{Answer: "最近在盤整", Usage: 500}, nil)
	fixture.conversationRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.Conversation{ID: 42}, nil)

	recorder := fixture.send(http.MethodPost, "/chat", `{"question":"BTCUSDT 最近走勢如何"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	answer := struct {
		ConversationID uint   `json:"conversationId"`
		Answer         string `json:"answer"`
	}{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
	assert.Equal(t, uint(42), answer.ConversationID)
	assert.Equal(t, "最近在盤整", answer.Answer)
}

func TestChatAskReportsABodyItCannotRead(t *testing.T) {
	fixture := newChatRouterUnderTest(t)

	recorder := fixture.send(http.MethodPost, "/chat", `{"question":`)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestChatAskMapsEachRefusalOntoWhatTheReaderMustDoAboutIt(t *testing.T) {
	testCases := []struct {
		name               string
		body               string
		expectedStatusCode int
		arrange            func(fixture chatRouterUnderTest)
	}{
		{
			name:               "a question that said nothing is the sender's to fix",
			body:               `{"question":"   "}`,
			expectedStatusCode: http.StatusBadRequest,
			arrange:            func(chatRouterUnderTest) {},
		},
		{
			name:               "a conversation that is not there",
			body:               `{"conversationId":99,"question":"BTCUSDT 最近走勢如何"}`,
			expectedStatusCode: http.StatusNotFound,
			arrange: func(fixture chatRouterUnderTest) {
				fixture.expectUsageToday(0)
				fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
					Return(entities.Conversation{}, domains.ConversationNotFound(99))
			},
		},
		{
			// Waiting is what this reader has to do, and it is a different thing from
			// trying again shortly — collapsing the two would leave somebody retrying
			// a refusal that will still be there in an hour.
			name:               "today's allowance is spent",
			body:               `{"question":"BTCUSDT 最近走勢如何"}`,
			expectedStatusCode: http.StatusTooManyRequests,
			arrange: func(fixture chatRouterUnderTest) {
				fixture.expectUsageToday(300000)
			},
		},
		{
			name:               "the assistant did not answer",
			body:               `{"question":"BTCUSDT 最近走勢如何"}`,
			expectedStatusCode: http.StatusServiceUnavailable,
			arrange: func(fixture chatRouterUnderTest) {
				fixture.expectUsageToday(0)
				fixture.assistantProxy.EXPECT().Reply(gomock.Any(), gomock.Any()).
					Return(vo.AssistantReplyVo{}, errors.New("dial tcp: connection refused"))
			},
		},
		{
			name:               "anything else is a failure of this system",
			body:               `{"question":"BTCUSDT 最近走勢如何"}`,
			expectedStatusCode: http.StatusBadGateway,
			arrange: func(fixture chatRouterUnderTest) {
				fixture.conversationRepository.EXPECT().
					SumUsageBetween(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(0, errors.New("storage unavailable"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newChatRouterUnderTest(t)
			testCase.arrange(fixture)

			recorder := fixture.send(http.MethodPost, "/chat", testCase.body)

			assert.Equal(t, testCase.expectedStatusCode, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "message")
		})
	}
}

func TestChatListConversationsAnswersWhateverIsThere(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.Conversation{
			{ID: 2, LastActiveAt: chatAskedAt},
			{ID: 1, LastActiveAt: chatAskedAt.Add(-time.Hour)},
		}, nil)

	recorder := fixture.send(http.MethodGet, "/chat/conversations", "")

	require.Equal(t, http.StatusOK, recorder.Code)
	summaries := make([]struct {
		ID uint `json:"id"`
	}, 0)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &summaries))
	require.Len(t, summaries, 2)
	assert.Equal(t, uint(2), summaries[0].ID)
}

func TestChatListConversationsAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.Conversation{}, nil)

	recorder := fixture.send(http.MethodGet, "/chat/conversations", "")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `[]`, recorder.Body.String())
}

func TestChatListConversationsReportsAFailureToRead(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.conversationRepository.EXPECT().FindAll(gomock.Any()).
		Return(nil, errors.New("storage unavailable"))

	recorder := fixture.send(http.MethodGet, "/chat/conversations", "")

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
}

func TestChatGetConversationHandsBackEveryMessage(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.Conversation{ID: 7, LastActiveAt: chatAskedAt, Turns: []entities.AssistantTurn{
			{Ask: "問 1", Answer: "答 1", CreatedAt: chatAskedAt},
		}}, nil)

	recorder := fixture.send(http.MethodGet, "/chat/conversations/7", "")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "問 1")
	assert.Contains(t, recorder.Body.String(), "答 1")
}

func TestChatGetConversationRefusesAnIdentifierThatIsNotOne(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{name: "not a number", path: "/chat/conversations/abc"},
		{name: "zero names no conversation", path: "/chat/conversations/0"},
		{name: "negative", path: "/chat/conversations/-1"},
		{name: "too large to be held", path: "/chat/conversations/99999999999999999999"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newChatRouterUnderTest(t)

			recorder := fixture.send(http.MethodGet, testCase.path, "")

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "對話識別碼必須是正整數")
		})
	}
}

func TestChatGetConversationReportsOneThatIsNotThere(t *testing.T) {
	fixture := newChatRouterUnderTest(t)
	fixture.conversationRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Conversation{}, domains.ConversationNotFound(99))

	recorder := fixture.send(http.MethodGet, "/chat/conversations/99", "")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
