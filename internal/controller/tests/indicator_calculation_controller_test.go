package controller_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const indicatorBody = `{"symbol":"BTCUSDT","candleCount":2,"script":"the script"}`

type indicatorRouterUnderTest struct {
	engine               *gin.Engine
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
}

func newIndicatorRouterUnderTest(t *testing.T) indicatorRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(mockController)

	indicatorCalculationController := controller.NewIndicatorCalculationController(
		application.NewIndicatorCalculationApplication(
			service.NewIndicatorCalculationService(
				kCandleRepository, indicatorScriptProxy, queryMaxResults)))

	engine := gin.New()
	engine.POST("/indicator-calculations", indicatorCalculationController.CalculateIndicator)

	return indicatorRouterUnderTest{
		engine: engine, kCandleRepository: kCandleRepository, indicatorScriptProxy: indicatorScriptProxy,
	}
}

func (fixture indicatorRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/indicator-calculations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)
	return recorder
}

func (fixture indicatorRouterUnderTest) expectTwoUsableCandles() {
	fixture.kCandleRepository.EXPECT().
		FindLatest("BTCUSDT", 3).
		Return([]entities.KCandle{
			kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100"),
		}, nil)
}

func TestCalculateIndicatorResponses(t *testing.T) {
	t.Run("reports success with the indicator values", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute("the script", gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"symbol":"BTCUSDT"`)
		assert.Contains(t, recorder.Body.String(), `"usedCandleCount":2`)
		assert.Contains(t, recorder.Body.String(), `"ma":110`)
	})

	t.Run("reports an empty set of values as success", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"values":{}`)
	})

	t.Run("reports a broken request as a bad request", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(`{"symbol":"BTCUSDT","candleCount":0,"script":"the script"}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "計算根數必須大於零")
	})

	t.Run("reports too few candles as a bad request naming what is usable", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatest("BTCUSDT", 3).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "可用 0 根")
	})

	t.Run("reports a script that cannot run as unprocessable", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	})

	t.Run("reports a storage failure as a bad gateway", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatest("BTCUSDT", 3).
			Return(nil, errors.New("storage unreachable"))

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("reports unreadable input as a bad request", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(`{"candleCount":`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestCalculateIndicatorReportsTheDeclaredResultType(t *testing.T) {
	t.Run("writes a series out as a series", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"line": {IsList: true, Numbers: []float64{100, 105}},
			}, nil)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","candleCount":2,"script":"the script","resultType":"floatList"}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"floatList"`)
		assert.Contains(t, recorder.Body.String(), `"line":[100,105]`)
	})

	t.Run("writes a lone answer out as an answer", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"crossed": {Booleans: []bool{false}}}, nil)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","candleCount":2,"script":"the script","resultType":"bool"}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"bool"`)
		assert.Contains(t, recorder.Body.String(), `"crossed":false`)
	})

	t.Run("declaring nothing still reports one number per indicator", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"float"`)
		assert.Contains(t, recorder.Body.String(), `"ma":110`)
	})

	t.Run("reports a kind that is not on offer as a bad request", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","candleCount":2,"script":"the script","resultType":"string"}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "指標值種類只能是")
	})
}
