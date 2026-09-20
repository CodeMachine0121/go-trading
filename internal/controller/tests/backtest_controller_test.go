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

// backtestRouterNow is the moment every request below is answered at, so that "up to
// when" is decided by the request rather than by whenever the suite runs.
var backtestRouterNow = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

var backtestRouterStart = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

const backtestBody = `{
	"symbol":"BTCUSDT",
	"aggregationInterval":"1h",
	"startTime":"2026-08-29T00:00:00Z",
	"endTime":"2026-08-29T04:00:00Z",
	"strategyScriptId":9,
	"initialCapital":"10000",
	"positionSizingMode":"allIn"
}`

// backtestRouterStrategyScriptID is the strategy script every replay below names. It belongs to
// the signed-in viewer and holds "the script".
const backtestRouterStrategyScriptID = uint(9)

type backtestRouterUnderTest struct {
	engine               *gin.Engine
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
}

func newBacktestRouterUnderTest(t *testing.T) backtestRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestRouterStrategyScriptID).
		Return(entities.StrategyScript{
			ID: backtestRouterStrategyScriptID, OwnerID: signedInViewerID, Script: "the script",
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	backtestController := controller.NewBacktestController(
		application.NewBacktestApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewBacktestService(
				kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults)))

	engine := gin.New()
	engine.POST("/backtests", doorOpenFor(t, signedInViewerID), backtestController.RunBacktest)

	return backtestRouterUnderTest{
		engine:               engine,
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func (fixture backtestRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/backtests", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// backtestRouterCandle builds a stored candle that many hours into the stretch.
func backtestRouterCandle(hour int, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: backtestRouterStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.RequireFromString(closePrice),
		High:     decimal.RequireFromString(closePrice),
		Low:      decimal.RequireFromString(closePrice),
		Close:    decimal.RequireFromString(closePrice),
	}
}

func (fixture backtestRouterUnderTest) expectTwoCandles() {
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			backtestRouterCandle(0, "100"), backtestRouterCandle(1, "110"),
		}, nil).AnyTimes()
}

func TestRunBacktestEndpoint(t *testing.T) {
	t.Run("a completed replay comes back with its report card", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil)

		response := fixture.post(backtestBody)

		require.Equal(t, http.StatusOK, response.Code)

		var body struct {
			Symbol   string `json:"symbol"`
			Interval string `json:"interval"`
			Summary  struct {
				FinalEquity       string   `json:"finalEquity"`
				TotalReturnRate   float64  `json:"totalReturnRate"`
				WinRate           *float64 `json:"winRate"`
				PositionOpenCount int      `json:"positionOpenCount"`
			} `json:"summary"`
			ClosedTrades []map[string]any `json:"closedTrades"`
			EquityCurve  []map[string]any `json:"equityCurve"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "BTCUSDT", body.Symbol)
		assert.Equal(t, "1h", body.Interval)
		assert.Equal(t, "11000", body.Summary.FinalEquity)
		assert.InDelta(t, 0.1, body.Summary.TotalReturnRate, 1e-9)
		assert.Equal(t, 1, body.Summary.PositionOpenCount)
		assert.Empty(t, body.ClosedTrades)
		assert.Len(t, body.EquityCurve, 2)
	})

	t.Run("a replay with nothing closed reports no win rate at all", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{{}, {}}, nil)

		response := fixture.post(backtestBody)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"winRate":null`)
	})

	t.Run("a body that is not readable is refused", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{"symbol":`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("conditions that cannot be replayed are the caller's to fix", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"0",
			"positionSizingMode":"allIn"
		}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "初始資金")
		// The input at fault travels as a value, so the sentence can be put beside the
		// box the person has to change rather than at the top of the page.
		assert.Contains(t, response.Body.String(), `"field":"initialCapital"`)
	})

	t.Run("a stretch that cannot be replayed names the time range", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{backtestRouterCandle(0, "100")}, nil)

		response := fixture.post(backtestBody)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), `"field":"timeRange"`)
	})

	t.Run("a sizing figure out of range names that figure", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"percentage",
			"positionSizingValue":"0"
		}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), `"field":"positionSizingValue"`)
	})

	t.Run("a stretch with too few candles is the caller's to fix", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{backtestRouterCandle(0, "100")}, nil)

		response := fixture.post(backtestBody)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("a knob nobody declared names the knob", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.UndeclaredParameter("period"))

		response := fixture.post(backtestBody)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), `"parameterName":"period"`)
	})

	t.Run("a script that could not run is answered as the script's fault", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		response := fixture.post(backtestBody)

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	})

	t.Run("storage refusing to answer is this system's fault", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, assert.AnError)

		response := fixture.post(backtestBody)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

// The mode a caller declares has to survive the whole way down to the account, and
// the refusal has to name the box the person types it into.
func TestRunBacktestEndpointCarriesTheTradingMode(t *testing.T) {
	t.Run("a long only replay stands aside instead of reversing", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalSell}},
			}, nil)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"allIn",
			"tradingMode":"spot"
		}`)

		require.Equal(t, http.StatusOK, response.Code)

		var body struct {
			Summary struct {
				FinalEquity       string `json:"finalEquity"`
				PositionOpenCount int    `json:"positionOpenCount"`
			} `json:"summary"`
			ClosedTrades []map[string]any `json:"closedTrades"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		// Bought at 100, sold at 110, now holding 11,000 in cash rather than a short.
		assert.Equal(t, "11000", body.Summary.FinalEquity)
		assert.Equal(t, 1, body.Summary.PositionOpenCount)
		assert.Len(t, body.ClosedTrades, 1)
	})

	t.Run("a trading mode nobody offers names the input at fault", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"allIn",
			"tradingMode":"dayTrade"
		}`)

		require.Equal(t, http.StatusBadRequest, response.Code)

		var body struct {
			Message string `json:"message"`
			Field   string `json:"field"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "tradingMode", body.Field)
		assert.Contains(t, body.Message, "longShort")
		assert.Contains(t, body.Message, "spot")
	})
}

func TestRunBacktestEndpointCarriesTheLeverage(t *testing.T) {
	t.Run("a borrowed replay moves the exposure rather than the stake", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"allIn",
			"leverage":"5"
		}`)

		require.Equal(t, http.StatusOK, response.Code)

		var body struct {
			Summary struct {
				FinalEquity          string `json:"finalEquity"`
				LiquidationExitCount int    `json:"liquidationExitCount"`
			} `json:"summary"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		// Bought at 100 and still holding at 110. Five times the exposure makes that
		// fifty percent rather than ten, so 15,000 rather than the 11,000 the same
		// body without a multiplier answers.
		assert.Equal(t, "15000", body.Summary.FinalEquity)
		assert.Equal(t, 0, body.Summary.LiquidationExitCount)
	})

	t.Run("spot has nobody to borrow from, and the refusal says where to look", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"allIn",
			"tradingMode":"spot",
			"leverage":"3"
		}`)

		require.Equal(t, http.StatusBadRequest, response.Code)

		var body struct {
			Message string `json:"message"`
			Field   string `json:"field"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "leverage", body.Field)
		assert.Contains(t, body.Message, "現貨")
	})

	t.Run("a maintenance margin leaving no room names the same input", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)

		response := fixture.post(`{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"strategyScriptId":9,
			"initialCapital":"10000",
			"positionSizingMode":"allIn",
			"leverage":"5",
			"maintenanceMarginRate":"25"
		}`)

		require.Equal(t, http.StatusBadRequest, response.Code)

		var body struct {
			Message string `json:"message"`
			Field   string `json:"field"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "leverage", body.Field)
		// The refusal says what would go through, so nobody has to guess their way to it.
		assert.Contains(t, body.Message, "20%")
	})

	t.Run("saying nothing answers exactly as it did before there was anything to say", func(t *testing.T) {
		fixture := newBacktestRouterUnderTest(t)
		fixture.expectTwoCandles()
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil)

		response := fixture.post(backtestBody)

		require.Equal(t, http.StatusOK, response.Code)

		var body struct {
			Summary struct {
				FinalEquity          string `json:"finalEquity"`
				LiquidationExitCount int    `json:"liquidationExitCount"`
			} `json:"summary"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "11000", body.Summary.FinalEquity)
		assert.Equal(t, 0, body.Summary.LiquidationExitCount)
	})
}
