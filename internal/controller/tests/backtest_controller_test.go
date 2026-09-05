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
	"script":"the script",
	"initialCapital":"10000",
	"positionSizingMode":"allIn"
}`

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

	backtestController := controller.NewBacktestController(
		application.NewBacktestApplication(
			service.NewBacktestService(
				kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults)))

	engine := gin.New()
	engine.POST("/backtests", backtestController.RunBacktest)

	return backtestRouterUnderTest{
		engine:               engine,
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func (fixture backtestRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/backtests", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
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
				{domains.SignalIndicatorName: {Numbers: []float64{1}}},
				{domains.SignalIndicatorName: {Numbers: []float64{0}}},
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
			"script":"the script",
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
			"script":"the script",
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
