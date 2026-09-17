package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
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

type tradingStrategyRouterUnderTest struct {
	engine                    *gin.Engine
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	strategyBotRepository     *mocks.MockIStrategyBotRepository
	strategyScriptRepository  *mocks.MockIStrategyScriptRepository
}

func newTradingStrategyRouterUnderTest(t *testing.T) tradingStrategyRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(mockController)
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(mockController)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(mockController)

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)).AnyTimes()

	tradingStrategyController := controller.NewTradingStrategyController(
		application.NewTradingStrategyApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository, clockProxy),
		))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/trading-strategies", requiresSignIn, tradingStrategyController.CreateTradingStrategy)
	engine.GET("/trading-strategies", requiresSignIn, tradingStrategyController.ListTradingStrategies)
	engine.GET("/trading-strategies/:id", requiresSignIn, tradingStrategyController.GetTradingStrategy)
	engine.PUT("/trading-strategies/:id", requiresSignIn, tradingStrategyController.UpdateTradingStrategy)
	engine.DELETE("/trading-strategies/:id", requiresSignIn, tradingStrategyController.DeleteTradingStrategy)

	return tradingStrategyRouterUnderTest{
		engine:                    engine,
		tradingStrategyRepository: tradingStrategyRepository,
		strategyBotRepository:     strategyBotRepository,
		strategyScriptRepository:  strategyScriptRepository,
	}
}

func (fixture tradingStrategyRouterUnderTest) send(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// aMixedCoarsenessTradingStrategyBody is the same trading strategy with its two
// sources reading different coarsenesses — the one shape that is well formed in every
// other way and still cannot be saved.
const aMixedCoarsenessTradingStrategyBody = `{
	"name": "黃金交叉",
	"signalSources": [
		{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"},
		{"label": "B", "strategyScriptId": 9, "aggregationInterval": "5m"}
	],
	"buyCondition": {"sourceLabel": "A", "signal": "buy"},
	"sellCondition": {"sourceLabel": "A", "signal": "sell"}
}`

// aTradingStrategyBody has a nested buy condition, so that the nesting a person
// builds on screen is proven to survive the journey in.
const aTradingStrategyBody = `{
	"name": "黃金交叉",
	"signalSources": [
		{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h",
		 "parameterValues": [{"name": "回看根數", "value": 20}]},
		{"label": "B", "strategyScriptId": 9, "aggregationInterval": "1h"}
	],
	"buyCondition": {
		"operator": "and",
		"conditions": [
			{"sourceLabel": "A", "signal": "buy"},
			{"sourceLabel": "B", "signal": "buy"}
		]
	},
	"sellCondition": {"sourceLabel": "A", "signal": "sell"}
}`

func (fixture tradingStrategyRouterUnderTest) expectResolvableStrategyScript() {
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.StrategyScript{
			ID: 9, OwnerID: signedInViewerID, Name: "均線", Script: "//", ResultType: "signal",
			Parameters: []entities.StrategyScriptParameter{
				{StrategyScriptID: 9, Name: "回看根數", Kind: "lookbackCount", DefaultValue: 20},
			},
		}, nil).AnyTimes()
}

func aStoredTradingStrategyRow() entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: 11, OwnerID: signedInViewerID, Name: "黃金交叉",
		SignalSources: []entities.TradingStrategySignalSource{
			{ID: 20, TradingStrategyID: 11, Label: "A", StrategyScriptID: 9, AggregationInterval: "1h"},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 10, TradingStrategyID: 11, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 11, TradingStrategyID: 11, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

func TestTradingStrategyRouterCreatesOneAndAnswersWithIt(t *testing.T) {
	fixture := newTradingStrategyRouterUnderTest(t)
	fixture.expectResolvableStrategyScript()

	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, tradingStrategy entities.TradingStrategy,
		) (entities.TradingStrategy, error) {
			// The nesting a person built on screen arrived intact.
			require.Len(t, tradingStrategy.ConditionNodes, 2)
			assert.Equal(t,
				string(vo.ConditionOperatorAnd), tradingStrategy.ConditionNodes[0].Operator)
			require.Len(t, tradingStrategy.ConditionNodes[0].Children, 2)
			// The owner comes from the proof of identity, never from the body.
			assert.Equal(t, signedInViewerID, tradingStrategy.OwnerID)

			return aStoredTradingStrategyRow(), nil
		})

	response := fixture.send(http.MethodPost, "/trading-strategies", aTradingStrategyBody)

	require.Equal(t, http.StatusCreated, response.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "黃金交叉", answer["name"])
	// The owner is nothing a person reading their own learns from.
	assert.NotContains(t, answer, "ownerId")
}

func TestTradingStrategyRouterListsAndReadsThem(t *testing.T) {
	fixture := newTradingStrategyRouterUnderTest(t)

	fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).
		Return([]entities.TradingStrategy{aStoredTradingStrategyRow()}, nil)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aStoredTradingStrategyRow(), nil)

	listResponse := fixture.send(http.MethodGet, "/trading-strategies", "")
	require.Equal(t, http.StatusOK, listResponse.Code)
	listed := []map[string]any{}
	require.NoError(t, json.Unmarshal(listResponse.Body.Bytes(), &listed))
	require.Len(t, listed, 1)

	readResponse := fixture.send(http.MethodGet, "/trading-strategies/11", "")
	require.Equal(t, http.StatusOK, readResponse.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(readResponse.Body.Bytes(), &answer))
	assert.Equal(t, "黃金交叉", answer["name"])
	// The two trees come back nested, in the shape a person wrote them.
	assert.NotNil(t, answer["buyCondition"])
	assert.NotNil(t, answer["sellCondition"])
}

func TestTradingStrategyRouterRewritesAndDeletes(t *testing.T) {
	fixture := newTradingStrategyRouterUnderTest(t)
	fixture.expectResolvableStrategyScript()

	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aStoredTradingStrategyRow(), nil).AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), uint(11)).
		Return([]entities.StrategyBot{}, nil).AnyTimes()
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aStoredTradingStrategyRow(), nil)
	fixture.tradingStrategyRepository.EXPECT().Delete(gomock.Any(), uint(11)).Return(nil)

	rewriteResponse := fixture.send(
		http.MethodPut, "/trading-strategies/11", aTradingStrategyBody)
	require.Equal(t, http.StatusOK, rewriteResponse.Code)

	deleteResponse := fixture.send(http.MethodDelete, "/trading-strategies/11", "")
	// Nothing to say back, so nothing is said back.
	assert.Equal(t, http.StatusNoContent, deleteResponse.Code)
}

func TestTradingStrategyRouterMapsEachRefusalOntoItsOwnStatus(t *testing.T) {
	testCases := []struct {
		name           string
		arrange        func(fixture tradingStrategyRouterUnderTest)
		method         string
		target         string
		body           string
		expectedStatus int
	}{
		{
			name:           "a body that breaks a rule is the caller's mistake",
			arrange:        func(fixture tradingStrategyRouterUnderTest) {},
			method:         http.MethodPost,
			target:         "/trading-strategies",
			body:           `{"name": "  "}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "somebody else's is not there",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				strangers := aStoredTradingStrategyRow()
				strangers.OwnerID = 4242
				fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
					Return(strangers, nil)
			},
			method:         http.MethodGet,
			target:         "/trading-strategies/11",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "a name this person already uses is a conflict",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				fixture.expectResolvableStrategyScript()
				fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.TradingStrategy{}, domains.ErrTradingStrategyNameConflict)
			},
			method:         http.MethodPost,
			target:         "/trading-strategies",
			body:           aTradingStrategyBody,
			expectedStatus: http.StatusConflict,
		},
		{
			name: "a rewrite while a following bot runs is a conflict",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
					Return(aStoredTradingStrategyRow(), nil)
				fixture.strategyBotRepository.EXPECT().
					FindAllByTradingStrategy(gomock.Any(), uint(11)).
					Return([]entities.StrategyBot{{
						ID: 3, OwnerID: signedInViewerID, Name: "幣安盯盤",
						TradingStrategyID: 11, RunState: string(vo.StrategyBotRunning),
					}}, nil)
			},
			method:         http.MethodPut,
			target:         "/trading-strategies/11",
			body:           aTradingStrategyBody,
			expectedStatus: http.StatusConflict,
		},
		{
			name: "a delete while any bot follows it is a conflict",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
					Return(aStoredTradingStrategyRow(), nil)
				fixture.strategyBotRepository.EXPECT().
					FindAllByTradingStrategy(gomock.Any(), uint(11)).
					Return([]entities.StrategyBot{{
						ID: 3, OwnerID: signedInViewerID, Name: "已停止的",
						TradingStrategyID: 11, RunState: string(vo.StrategyBotStopped),
					}}, nil)
			},
			method:         http.MethodDelete,
			target:         "/trading-strategies/11",
			expectedStatus: http.StatusConflict,
		},
		{
			name: "a script the caller cannot see is not there",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				strangersScript := entities.StrategyScript{
					ID: 9, OwnerID: 4242, Name: "別人的", Script: "//", ResultType: "signal",
				}
				fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
					Return(strangersScript, nil)
			},
			method:         http.MethodPost,
			target:         "/trading-strategies",
			body:           aTradingStrategyBody,
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "sources reading different coarsenesses are the caller's mistake",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				fixture.expectResolvableStrategyScript()
			},
			method:         http.MethodPost,
			target:         "/trading-strategies",
			body:           aMixedCoarsenessTradingStrategyBody,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "storage that could not answer is not the caller's fault",
			arrange: func(fixture tradingStrategyRouterUnderTest) {
				fixture.tradingStrategyRepository.EXPECT().
					FindAllByOwner(gomock.Any(), signedInViewerID).
					Return(nil, errors.New("the database went away"))
			},
			method:         http.MethodGet,
			target:         "/trading-strategies",
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newTradingStrategyRouterUnderTest(t)
			testCase.arrange(fixture)

			response := fixture.send(testCase.method, testCase.target, testCase.body)

			assert.Equal(t, testCase.expectedStatus, response.Code)
		})
	}
}

func TestTradingStrategyRouterRefusesAnUnreadableIdentifier(t *testing.T) {
	fixture := newTradingStrategyRouterUnderTest(t)

	for _, target := range []string{"/trading-strategies/abc", "/trading-strategies/0"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			response := fixture.send(method, target, aTradingStrategyBody)

			assert.Equal(t, http.StatusBadRequest, response.Code, "%s %s", method, target)
		}
	}
}

func TestTradingStrategyRouterRefusesAnUnreadableBody(t *testing.T) {
	fixture := newTradingStrategyRouterUnderTest(t)

	response := fixture.send(http.MethodPost, "/trading-strategies", `{"name":`)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
