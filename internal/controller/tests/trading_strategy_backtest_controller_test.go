package controller_test

import (
	"encoding/json"
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
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const tradingStrategyBacktestBody = `{
	"symbol":"BTCUSDT",
	"startTime":"2026-08-29T00:00:00Z",
	"endTime":"2026-08-29T04:00:00Z",
	"initialCapital":"10000",
	"positionSizingMode":"allIn"
}`

type tradingStrategyBacktestRouterUnderTest struct {
	engine                    *gin.Engine
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	runs                      routerBacktestRuns
}

func newTradingStrategyBacktestRouterUnderTest(t *testing.T) tradingStrategyBacktestRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(mockController)

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.StrategyScript{
			ID: 9, OwnerID: signedInViewerID, Script: "the script",
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	backtestService := service.NewBacktestService(
		mocks.NewMockIKCandleRepository(mockController), mocks.NewMockIIndicatorScriptProxy(mockController),
		clockProxy, 1000, 0)
	runs := newRouterBacktestRuns(t, mockController, clockProxy, backtestService, nil)
	tradingStrategyBacktestController := controller.NewTradingStrategyBacktestController(
		application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			backtestService, nil, runs.backtestRunService, time.Minute,
		))

	engine := gin.New()
	engine.POST("/trading-strategies/:id/backtests", doorOpenFor(t, signedInViewerID),
		tradingStrategyBacktestController.SubmitTradingStrategyBacktest)

	return tradingStrategyBacktestRouterUnderTest{
		engine:                    engine,
		tradingStrategyRepository: tradingStrategyRepository,
		runs:                      runs,
	}
}

func (fixture tradingStrategyBacktestRouterUnderTest) send(
	target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// aRoutedTradingStrategy reads one source at the given interval, buying on its buy and selling on its sell.
func aRoutedTradingStrategy(interval string) entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: 11, OwnerID: signedInViewerID, Name: "黃金交叉",
		SignalSources: []entities.TradingStrategySignalSource{
			{ID: 20, TradingStrategyID: 11, Label: "A",
				StrategyScriptID: 9, AggregationInterval: interval},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 10, TradingStrategyID: 11, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 13, TradingStrategyID: 11, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

func TestTradingStrategyBacktestRouterQueuesTheStrategyAndAnswersWithTheRun(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)
	saved := fixture.runs.expectQueued()

	response := fixture.send("/trading-strategies/11/backtests", tradingStrategyBacktestBody)

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "spotTradingStrategy", answer["kind"])
	assert.Equal(t, "queued", answer["status"])
	// The interval is returned even though none was sent, since it comes from the trading strategy.
	assert.Equal(t, "1h", answer["interval"])
	assert.Equal(t, float64(11), answer["tradingStrategyId"])
	assert.Equal(t, "黃金交叉", answer["subjectName"])
	assert.Nil(t, answer["leverage"])
	assert.NotContains(t, answer, "summary")
	require.NotNil(t, saved.Request.SpotTradingStrategy)
	require.Len(t, saved.Request.SpotTradingStrategy.SignalSources, 1)
	assert.Equal(t, "the script", saved.Request.SpotTradingStrategy.SignalSources[0].Script)
}

func TestTradingStrategyBacktestRouterMapsEachRefusalOntoItsOwnStatus(t *testing.T) {
	testCases := []struct {
		name           string
		arrange        func(fixture tradingStrategyBacktestRouterUnderTest)
		target         string
		body           string
		expectedStatus int
	}{
		{
			name:           "an unreadable identifier is the caller's mistake",
			arrange:        func(fixture tradingStrategyBacktestRouterUnderTest) {},
			target:         "/trading-strategies/abc/backtests",
			body:           tradingStrategyBacktestBody,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "an unreadable body is too",
			arrange:        func(fixture tradingStrategyBacktestRouterUnderTest) {},
			target:         "/trading-strategies/11/backtests",
			body:           `{"symbol":`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "somebody else's trading strategy is not there",
			arrange: func(fixture tradingStrategyBacktestRouterUnderTest) {
				strangers := aRoutedTradingStrategy("1h")
				strangers.OwnerID = signedInViewerID + 1
				fixture.tradingStrategyRepository.EXPECT().
					FindOne(gomock.Any(), uint(11)).Return(strangers, nil)
			},
			target:         "/trading-strategies/11/backtests",
			body:           tradingStrategyBacktestBody,
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "a source reading an interval outside the six names the input at fault",
			arrange: func(fixture tradingStrategyBacktestRouterUnderTest) {
				disagreeing := aRoutedTradingStrategy("1h")
				disagreeing.SignalSources = append(disagreeing.SignalSources,
					entities.TradingStrategySignalSource{
						ID: 21, TradingStrategyID: 11, Label: "B",
						StrategyScriptID: 9, AggregationInterval: "3h",
					})
				fixture.tradingStrategyRepository.EXPECT().
					FindOne(gomock.Any(), uint(11)).Return(disagreeing, nil)
			},
			target:         "/trading-strategies/11/backtests",
			body:           tradingStrategyBacktestBody,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "a capital of nothing is the caller's mistake",
			arrange: func(fixture tradingStrategyBacktestRouterUnderTest) {
				fixture.tradingStrategyRepository.EXPECT().
					FindOne(gomock.Any(), uint(11)).Return(aRoutedTradingStrategy("1h"), nil)
			},
			target: "/trading-strategies/11/backtests",
			body: `{"symbol":"BTCUSDT","startTime":"2026-08-29T00:00:00Z",` +
				`"endTime":"2026-08-29T04:00:00Z","initialCapital":"0",` +
				`"positionSizingMode":"allIn"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newTradingStrategyBacktestRouterUnderTest(t)
			testCase.arrange(fixture)
			// The request's own fault is reported even at the owner limit.
			fixture.runs.ownerHasActive(routerBacktestRunOwnerActiveLimit)

			response := fixture.send(testCase.target, testCase.body)

			assert.Equal(t, testCase.expectedStatus, response.Code)
		})
	}
}

// The field is named so the message can be shown beside that input.
func TestTradingStrategyBacktestRouterNamesTheInputAtFault(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	disagreeing := aRoutedTradingStrategy("1h")
	disagreeing.SignalSources = append(disagreeing.SignalSources,
		entities.TradingStrategySignalSource{
			ID: 21, TradingStrategyID: 11, Label: "B",
			StrategyScriptID: 9, AggregationInterval: "3h",
		})
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(11)).Return(disagreeing, nil)

	response := fixture.send("/trading-strategies/11/backtests", tradingStrategyBacktestBody)

	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "signalSources", answer["field"])
	assert.Contains(t, answer["message"], `信號來源 "B" 的彙總刻度不對`)
	assert.Contains(t, answer["message"], "1m、5m、15m、1h、4h、1d")
}

func TestTradingStrategyBacktestRouterAtTheOwnerLimit(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)
	fixture.runs.ownerHasActive(routerBacktestRunOwnerActiveLimit)

	response := fixture.send("/trading-strategies/11/backtests", tradingStrategyBacktestBody)

	require.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.JSONEq(t, `{"message":"你同時最多 3 筆回測在排隊或進行中，等其中一筆結束再送","ownerActiveLimit":3}`,
		response.Body.String())
}

// Another mode is refused here too, since an undeclared field would otherwise be silently dropped and a spot result returned.
func TestTradingStrategyBacktestRouterRefusesAnotherSetOfRules(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)

	response := fixture.send("/trading-strategies/11/backtests", `{
		"symbol":"BTCUSDT",
		"startTime":"2026-08-29T00:00:00Z",
		"endTime":"2026-08-29T04:00:00Z",
		"initialCapital":"10000",
		"positionSizingMode":"allIn",
		"tradingMode":"longShort"
	}`)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "只重演現貨")
}

// Borrowing is refused with the same message as a script replay, because both use the same gate.
func TestTradingStrategyBacktestRouterRefusesBorrowing(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)

	response := fixture.send("/trading-strategies/11/backtests", `{
		"symbol":"BTCUSDT",
		"startTime":"2026-08-29T00:00:00Z",
		"endTime":"2026-08-29T04:00:00Z",
		"initialCapital":"10000",
		"positionSizingMode":"allIn",
		"leverage":"5"
	}`)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "沒有人借錢給你")
}

// A maintenance margin rate is refused because an undeclared field would be silently dropped and a spot result returned.
func TestTradingStrategyBacktestRouterRefusesAMaintenanceMarginRate(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)

	response := fixture.send("/trading-strategies/11/backtests", `{
		"symbol":"BTCUSDT",
		"startTime":"2026-08-29T00:00:00Z",
		"endTime":"2026-08-29T04:00:00Z",
		"initialCapital":"10000",
		"positionSizingMode":"allIn",
		"maintenanceMarginRate":"0.5"
	}`)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "沒有維持保證金率")
}
