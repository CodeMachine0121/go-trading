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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
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
	kCandleRepository         *mocks.MockIKCandleRepository
	indicatorScriptProxy      *mocks.MockIIndicatorScriptProxy
}

func newTradingStrategyBacktestRouterUnderTest(t *testing.T) tradingStrategyBacktestRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(mockController)

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

	tradingStrategyBacktestController := controller.NewTradingStrategyBacktestController(
		application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewBacktestService(
				kCandleRepository, indicatorScriptProxy, clockProxy, 1000),
		))

	engine := gin.New()
	engine.POST("/trading-strategies/:id/backtests", doorOpenFor(t, signedInViewerID),
		tradingStrategyBacktestController.RunTradingStrategyBacktest)

	return tradingStrategyBacktestRouterUnderTest{
		engine:                    engine,
		tradingStrategyRepository: tradingStrategyRepository,
		kCandleRepository:         kCandleRepository,
		indicatorScriptProxy:      indicatorScriptProxy,
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

// aRoutedTradingStrategy is one source reading hourly candles, buying on its buy and
// selling on its sell.
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

func aRoutedHourlyCandle(hour int, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: backtestRouterStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.RequireFromString(closePrice),
		High:     decimal.RequireFromString(closePrice),
		Low:      decimal.RequireFromString(closePrice),
		Close:    decimal.RequireFromString(closePrice),
	}
}

func TestTradingStrategyBacktestRouterReplaysAndAnswersWithTheReport(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			aRoutedHourlyCandle(0, "100"), aRoutedHourlyCandle(1, "110"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil)

	response := fixture.send("/trading-strategies/11/backtests", tradingStrategyBacktestBody)

	require.Equal(t, http.StatusOK, response.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	// The coarseness comes back even though nobody sent one: it is the trading
	// strategy's answer, and whoever draws the curve has to know which it was.
	assert.Equal(t, "1h", answer["interval"])
	summary, isObject := answer["summary"].(map[string]any)
	require.True(t, isObject)
	assert.Equal(t, float64(0), summary["conflictedCandleCount"])
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
			name: "sources that disagree about coarseness name the input at fault",
			arrange: func(fixture tradingStrategyBacktestRouterUnderTest) {
				disagreeing := aRoutedTradingStrategy("1h")
				disagreeing.SignalSources = append(disagreeing.SignalSources,
					entities.TradingStrategySignalSource{
						ID: 21, TradingStrategyID: 11, Label: "B",
						StrategyScriptID: 9, AggregationInterval: "5m",
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

			response := fixture.send(testCase.target, testCase.body)

			assert.Equal(t, testCase.expectedStatus, response.Code)
		})
	}
}

// The refusal about coarseness names the field somebody has to go and change, so the
// sentence can be put beside it rather than at the top of a page.
func TestTradingStrategyBacktestRouterNamesTheInputAtFault(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)
	disagreeing := aRoutedTradingStrategy("1h")
	disagreeing.SignalSources = append(disagreeing.SignalSources,
		entities.TradingStrategySignalSource{
			ID: 21, TradingStrategyID: 11, Label: "B",
			StrategyScriptID: 9, AggregationInterval: "5m",
		})
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(11)).Return(disagreeing, nil)

	response := fixture.send("/trading-strategies/11/backtests", tradingStrategyBacktestBody)

	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "signalSources", answer["field"])
	assert.Contains(t, answer["message"], "1h")
	assert.Contains(t, answer["message"], "5m")
}

// Replaying a whole set of rules trades the one way this system replays. A body still
// naming a set of rules of its own is refused rather than quietly ignored.
func TestTradingStrategyBacktestRouterTradesSpot(t *testing.T) {
	fixture := newTradingStrategyBacktestRouterUnderTest(t)

	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(11)).
		Return(aRoutedTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			aRoutedHourlyCandle(0, "100"), aRoutedHourlyCandle(1, "120"),
			aRoutedHourlyCandle(2, "90"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalSell}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil)

	response := fixture.send("/trading-strategies/11/backtests", `{
		"symbol":"BTCUSDT",
		"startTime":"2026-08-29T00:00:00Z",
		"endTime":"2026-08-29T04:00:00Z",
		"initialCapital":"10000",
		"positionSizingMode":"allIn"
	}`)

	require.Equal(t, http.StatusOK, response.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	summary, isObject := answer["summary"].(map[string]any)
	require.True(t, isObject)
	// Sold at 120 and stood aside for the fall to 90.
	assert.Equal(t, "12000", summary["finalEquity"])
	assert.Equal(t, float64(1), summary["positionOpenCount"])
}

// Asking to borrow is refused here in the same words a strategy-script replay refuses
// it, because both ask the same gate.
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
