package controller_test

import (
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
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const queryMaxResults = 1000

func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

func kCandleAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: openTime,
		Open:  decimal.RequireFromString("100"),
		High:  decimal.RequireFromString("120"),
		Low:   decimal.RequireFromString("90"),
		Close: decimal.RequireFromString(closePrice),
	}
}

const validBody = `{"symbol":"BTCUSDT","openTime":"2026-08-29T09:00:00Z",
"open":"100","high":"120","low":"90","close":"120",
"volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600"}`

type routerUnderTest struct {
	engine            *gin.Engine
	kCandleRepository *mocks.MockIKCandleRepository
}

func newRouterUnderTest(t *testing.T) routerUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()

	kCandleController := controller.NewKCandleController(application.NewKCandleApplication(
		service.NewKCandleService(kCandleRepository, clockProxy, queryMaxResults)))

	engine := gin.New()
	engine.POST("/k-candles", kCandleController.CreateKCandle)
	engine.GET("/k-candles", kCandleController.GetKCandlesInRange)
	engine.GET("/k-candles/:symbol/:openTime", kCandleController.GetKCandle)
	engine.PUT("/k-candles/:symbol/:openTime", kCandleController.UpdateKCandle)
	engine.DELETE("/k-candles/:symbol/:openTime", kCandleController.DeleteKCandle)

	return routerUnderTest{engine: engine, kCandleRepository: kCandleRepository}
}

func (fixture routerUnderTest) call(method string, target string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)
	return recorder
}

func TestCreateKCandleResponses(t *testing.T) {
	t.Run("reports success and echoes the stored candle", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().Save(gomock.Any()).Return(kCandleAt(at(9, 0), "120"), nil)

		recorder := fixture.call(http.MethodPost, "/k-candles", validBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"symbol":"BTCUSDT"`)
		assert.Contains(t, recorder.Body.String(), `"close":"120"`)
	})

	t.Run("reports a broken rule as a bad request naming the rule", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		offMarkBody := strings.Replace(validBody, "T09:00:00Z", "T09:03:00Z", 1)

		recorder := fixture.call(http.MethodPost, "/k-candles", offMarkBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "起始時間必須落在5分鐘刻度上")
	})

	t.Run("reports unreadable input as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodPost, "/k-candles", `{"openTime":"not a time"}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("reports a storage failure as a bad gateway", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Save(gomock.Any()).
			Return(entities.KCandle{}, errors.New("storage unreachable"))

		recorder := fixture.call(http.MethodPost, "/k-candles", validBody)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestGetKCandlesInRangeResponses(t *testing.T) {
	t.Run("returns the candles found in the range", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), queryMaxResults+1).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)

		recorder := fixture.call(http.MethodGet,
			"/k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:10:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"openTime":"2026-08-29T09:00:00Z"`)
	})

	t.Run("returns an empty list when the range holds nothing", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), queryMaxResults+1).
			Return([]entities.KCandle{}, nil)

		recorder := fixture.call(http.MethodGet,
			"/k-candles?symbol=BTCUSDT&startTime=2026-08-29T11:00:00Z&endTime=2026-08-29T12:00:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "[]", recorder.Body.String())
	})

	t.Run("reports a missing trading symbol as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodGet,
			"/k-candles?startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:10:00Z", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "必須指定交易標的")
	})

	t.Run("reports an unreadable time as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodGet, "/k-candles?symbol=BTCUSDT&startTime=yesterday&endTime=today", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "startTime")
	})

	t.Run("reports an unreadable end time as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodGet,
			"/k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=today", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "endTime")
	})
}

func TestNamedKCandleResponses(t *testing.T) {
	t.Run("returns the named candle", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindOne("BTCUSDT", at(9, 0)).Return(kCandleAt(at(9, 0), "110"), nil)

		recorder := fixture.call(http.MethodGet, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"close":"110"`)
	})

	t.Run("reports a candle that does not exist as not found", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindOne("BTCUSDT", at(9, 0)).
			Return(entities.KCandle{}, domains.ErrKCandleNotFound)

		recorder := fixture.call(http.MethodGet, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("reports an unreadable open time in the path as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodGet, "/k-candles/BTCUSDT/yesterday", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "openTime")
	})

	t.Run("updates the named candle", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().Update(gomock.Any()).Return(kCandleAt(at(9, 0), "120"), nil)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", validBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"close":"120"`)
	})

	t.Run("takes the candle it updates from the path, not the body", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		bodyWithoutIdentity := `{"open":"100","high":"120","low":"90","close":"120",
"volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600"}`
		fixture.kCandleRepository.EXPECT().
			Update(gomock.Any()).
			DoAndReturn(func(kCandle entities.KCandle) (entities.KCandle, error) {
				assert.Equal(t, "BTCUSDT", kCandle.Symbol)
				assert.Equal(t, at(9, 0), kCandle.OpenTime)
				return kCandleAt(at(9, 0), "120"), nil
			})

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", bodyWithoutIdentity)

		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("reports updating a candle that does not exist as not found", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Update(gomock.Any()).
			Return(entities.KCandle{}, domains.ErrKCandleNotFound)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", validBody)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("refuses an update that would move the candle to another open time", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		movingBody := strings.Replace(validBody, "T09:00:00Z", "T09:05:00Z", 1)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", movingBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "不得更換交易標的與起始時間")
	})

	t.Run("refuses an update that would move the candle to another trading symbol", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		movingBody := strings.Replace(validBody, `"symbol":"BTCUSDT"`, `"symbol":"ETHUSDT"`, 1)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", movingBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "不得更換交易標的與起始時間")
	})

	t.Run("reports unreadable input on update as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", `{"open":`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("reports an unreadable open time on update as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodPut, "/k-candles/BTCUSDT/yesterday", validBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("removes the named candle and returns no content", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().Delete("BTCUSDT", at(9, 0)).Return(nil)

		recorder := fixture.call(http.MethodDelete, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("reports deleting a candle that does not exist as not found", func(t *testing.T) {
		fixture := newRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Delete("BTCUSDT", at(9, 0)).
			Return(domains.ErrKCandleNotFound)

		recorder := fixture.call(http.MethodDelete, "/k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("reports an unreadable open time on delete as a bad request", func(t *testing.T) {
		fixture := newRouterUnderTest(t)

		recorder := fixture.call(http.MethodDelete, "/k-candles/BTCUSDT/yesterday", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}
