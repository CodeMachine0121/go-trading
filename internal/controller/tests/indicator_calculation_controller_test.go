package controller_test

import (
	"errors"
	"fmt"
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
	"go.uber.org/mock/gomock"
)

// indicatorBody covers the two minutes ending at indicatorRouterNow, i.e. two one-minute slots.
const indicatorBody = `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","strategyScriptId":9}`

// indicatorRouterNow pins the request time so "up to when" doesn't depend on when the suite runs.
var indicatorRouterNow = at(9, 15)

// indicatorRouterStrategyScriptID belongs to the signed-in viewer.
const indicatorRouterStrategyScriptID = uint(9)

// indicatorRouterStrategyScriptStore always resolves that one script; its access gates are tested elsewhere.
func indicatorRouterStrategyScriptStore(
	mockController *gomock.Controller, declaredResultType string,
) *service.StrategyScriptService {
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), indicatorRouterStrategyScriptID).
		Return(entities.StrategyScript{
			ID: indicatorRouterStrategyScriptID, OwnerID: signedInViewerID, Script: "the script",
			ResultType: declaredResultType,
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	return service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)
}

type indicatorRouterUnderTest struct {
	engine               *gin.Engine
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
}

// newIndicatorRouterUnderTest runs a script that declares no result type, as most cases need.
func newIndicatorRouterUnderTest(t *testing.T) indicatorRouterUnderTest {
	return newIndicatorRouterRunning(t, "")
}

// newIndicatorRouterRunning runs a script declaring this result type, since the kind is declared on the script, not the request.
func newIndicatorRouterRunning(t *testing.T, declaredResultType string) indicatorRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	// Every symbol here trades around the clock unless a case says otherwise.
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(indicatorRouterNow).AnyTimes()

	indicatorCalculationController := controller.NewIndicatorCalculationController(
		application.NewIndicatorCalculationApplication(
			indicatorRouterStrategyScriptStore(mockController, declaredResultType),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
				domains.NewMarketCatalogDomain(
					map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				queryMaxResults),
			// Nothing here calculates over contract bars; that route has its own tests.
			nil))

	engine := gin.New()
	engine.POST("/indicator-calculations",
		doorOpenFor(t, signedInViewerID), indicatorCalculationController.CalculateIndicator)

	return indicatorRouterUnderTest{
		engine: engine, kCandleRepository: kCandleRepository, indicatorScriptProxy: indicatorScriptProxy,
	}
}

func (fixture indicatorRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/indicator-calculations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)
	return recorder
}

func (fixture indicatorRouterUnderTest) expectTwoUsableCandles() {
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorRouterNow, 3).
		Return([]entities.KCandle{
			kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100"),
		}, nil)
}

func TestCalculateIndicatorResponses(t *testing.T) {
	t.Run("reports success with the indicator values", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"symbol":"BTCUSDT"`)
		assert.Contains(t, recorder.Body.String(), `"usedCandleCount":2`)
		assert.Contains(t, recorder.Body.String(), `"ma":110`)
		assert.Contains(t, recorder.Body.String(), `"interval":"1m"`)
		assert.Contains(t, recorder.Body.String(), `"openTimes":["2026-08-29T09:05:00Z","2026-08-29T09:10:00Z"]`,
			"呼叫端要把值擺回圖上，就得知道這次讀的是哪幾根")
	})

	t.Run("reads at the coarseness and up to the moment the body named", func(t *testing.T) {
		// One hour is twelve 5m candles, so two buckets plus the spare reads 36; an end time of 14:30 stops at 14:00 since that hour is unfinished.
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", time.Date(2025, 3, 1, 14, 0, 0, 0, time.UTC), 180).
			Return([]entities.KCandle{
				kCandleAt(time.Date(2025, 3, 1, 13, 0, 0, 0, time.UTC), "100"),
				kCandleAt(time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC), "100"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		recorder := fixture.post(`{"symbol":"BTCUSDT","aggregationInterval":"1h",` +
			`"startTime":"2025-03-01T12:30:00Z","endTime":"2025-03-01T14:30:00Z",` +
			`"strategyScriptId":9}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"interval":"1h"`)
		assert.Contains(t, recorder.Body.String(),
			`"openTimes":["2025-03-01T12:00:00Z","2025-03-01T13:00:00Z"]`)
	})

	t.Run("reports an interval nobody offers as a bad request", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","aggregationInterval":"7m",` +
				`"startTime":"2026-08-29T09:13:00Z","strategyScriptId":9}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "彙總刻度只能是")
	})

	t.Run("reports an empty set of values as success", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"values":{}`)
	})

	t.Run("reports a broken request as a bad request", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:15:00Z","strategyScriptId":9}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "起點必須早於終點")
	})

	t.Run("answers a short stretch with both counts rather than refusing", func(t *testing.T) {
		// Two buckets asked for, one stored: a success carrying the shortfall so the caller can explain the shorter line.
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorRouterNow, 3).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"requiredCandleCount":2`)
		assert.Contains(t, recorder.Body.String(), `"usedCandleCount":1`)
	})

	t.Run("reports a stretch too thin to answer, handing over both counts", func(t *testing.T) {
		// The two numbers are returned as fields so callers don't have to parse the message.
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorRouterNow, 3).
			Return(nil, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"availableCandleCount":0`)
		assert.Contains(t, recorder.Body.String(), `"minimumCandleCount":1`)
	})

	t.Run("keeps asking for too much apart from a stretch too thin", func(t *testing.T) {
		// Their remedies are opposite (finer vs coarser), so callers must tell them apart; neither reaches storage.
		fixture := newIndicatorRouterUnderTest(t)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","startTime":"2020-01-01T00:00:00Z","strategyScriptId":9}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"field":"startTime"`)
		assert.NotContains(t, recorder.Body.String(), "availableCandleCount")
	})

	t.Run("reports a script that cannot run as unprocessable", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	})

	t.Run("reports every compartment staying busy as unavailable for now", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("%w: every slot stayed taken", domains.ErrIndicatorScriptCompartmentsBusy))

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"compartmentsBusy":true`)
	})

	t.Run("reports a storage failure as a bad gateway", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorRouterNow, 3).
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

// 每種失敗依「使用者該改哪裡」分開回答；名稱對不上是請求本身的錯，不是系統故障。
func TestCalculateIndicatorTellsAMismatchedParameterNameApartFromEverythingElse(t *testing.T) {
	fixture := newIndicatorRouterUnderTest(t)
	fixture.expectTwoUsableCandles()
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, domains.UndeclaredParameter("期數"))

	recorder := fixture.post(indicatorBody)

	assert.Equal(t, http.StatusBadRequest, recorder.Code,
		"這是呼叫端填錯了，不是這個系統壞了")
	assert.Contains(t, recorder.Body.String(), `"parameterName":"期數"`,
		"名字要以一個欄位交出去——靠讀訊息比對，等於讓呼叫端依賴給人看的文字")
}

// 這種失敗的出路是縮短區間或換粗刻度，呼叫端需從欄位而非句子辨識它。
func TestCalculateIndicatorNamesTheInputWhenTheSpanNeedsMoreCandlesThanOneCallMayRead(t *testing.T) {
	fixture := newIndicatorRouterUnderTest(t)

	// 一分鐘刻度下，超過上限的分鐘數就是超過上限的格數。
	recorder := fixture.post(
		`{"symbol":"BTCUSDT","startTime":"` +
			indicatorRouterNow.Add(-time.Duration(queryMaxResults+1)*time.Minute).
				Format(time.RFC3339) +
			`","strategyScriptId":9}`)

	assert.Equal(t, http.StatusBadRequest, recorder.Code,
		"這是呼叫端要的太多了，不是這個系統壞了")
	assert.Contains(t, recorder.Body.String(), `"field":"startTime"`,
		"是哪一格出的問題要以一個欄位交出去，呼叫端才擺得到那一格旁邊")
	assert.Contains(t, recorder.Body.String(), "超過單次可用的最大根數",
		"人讀的那一句仍然說得出用到幾根與上限是多少")
}

func TestCalculateIndicatorReportsTheDeclaredResultType(t *testing.T) {
	t.Run("writes a series out as a series", func(t *testing.T) {
		fixture := newIndicatorRouterRunning(t, "floatList")
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"line": {IsList: true, Numbers: []float64{100, 105}},
			}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"floatList"`)
		assert.Contains(t, recorder.Body.String(), `"line":[100,105]`)
	})

	t.Run("writes a lone answer out as an answer", func(t *testing.T) {
		fixture := newIndicatorRouterRunning(t, "bool")
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"crossed": {Booleans: []bool{false}}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"bool"`)
		assert.Contains(t, recorder.Body.String(), `"crossed":false`)
	})

	t.Run("declaring nothing still reports one number per indicator", func(t *testing.T) {
		fixture := newIndicatorRouterUnderTest(t)
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"float"`)
		assert.Contains(t, recorder.Body.String(), `"ma":110`)
	})

	t.Run("writes a signal out as the result itself", func(t *testing.T) {
		fixture := newIndicatorRouterRunning(t, "signal")
		fixture.expectTwoUsableCandles()
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, nil)

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"resultType":"signal"`)
		assert.Contains(t, recorder.Body.String(), `"signal":"buy"`)
	})

	t.Run("reports a kind that is not on offer as a bad request", func(t *testing.T) {
		// The kind comes from the stored script, so an unknown kind is a corrupt row and is still refused.
		fixture := newIndicatorRouterRunning(t, "string")

		recorder := fixture.post(indicatorBody)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "指標值種類只能是")
		assert.Contains(t, recorder.Body.String(), "signal")
	})
}

// Calculating an indicator also names a symbol, so an unstorable one must be refused here rather than reported as a server failure.
func TestIndicatorCalculationRouterRefusesAnUnstorableSymbol(t *testing.T) {
	// No expectation is set on the repository: nothing may reach storage.
	fixture := newIndicatorRouterUnderTest(t)

	recorder := fixture.post(
		`{"symbol":"BTC\u0000USDT","startTime":"2026-08-29T09:12:00Z","strategyScriptId":9}`)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "NUL")
}

// 「沒有交易」與「要太多」「湊得太薄」的出路不同（換刻度不會讓週六有成交），呼叫端須能分辨。
func TestCalculateIndicatorNamesAStretchThatHoldsNoMarketAsItsOwnKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
		Return(entities.TradingSymbol{Symbol: "2330", Market: string(vo.MarketTaiwanStock)}, true, nil)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)).AnyTimes()

	indicatorCalculationController := controller.NewIndicatorCalculationController(
		application.NewIndicatorCalculationApplication(
			indicatorRouterStrategyScriptStore(mockController, ""),
			service.NewIndicatorCalculationService(
				mocks.NewMockIKCandleRepository(mockController), tradingSymbolRepository,
				mocks.NewMockIIndicatorScriptProxy(mockController), clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
					vo.MarketTaiwanStock: {
						TradingSession: vo.TradingSessionVo{
							Location:   time.FixedZone("Asia/Taipei", 8*60*60),
							DailyStart: 9 * time.Hour,
							DailyEnd:   13*time.Hour + 30*time.Minute,
							Weekdays: []time.Weekday{
								time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
							},
						},
					},
				}),
				queryMaxResults),
			// Nothing here calculates over contract bars; that route has its own tests.
			nil))

	engine := gin.New()
	engine.POST("/indicator-calculations",
		doorOpenFor(t, signedInViewerID), indicatorCalculationController.CalculateIndicator)

	// 2026-09-12 是週六，整天都沒有交易。
	request := httptest.NewRequest(http.MethodPost, "/indicator-calculations", strings.NewReader(
		`{"symbol":"2330","startTime":"2026-09-12T00:00:00+08:00",`+
			`"endTime":"2026-09-13T00:00:00+08:00","strategyScriptId":9}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"observationWindowHoldsNoTrading":true`,
		"呼叫端要分辨得出這一種，才給得出「改看有交易的時間」這條出路")
	assert.Contains(t, recorder.Body.String(), "沒有交易")
	assert.NotContains(t, recorder.Body.String(), "availableCandleCount",
		"這不是湊得太薄——兩者的出路正好相反")
}

func TestCalculateIndicatorTurnsAwayARequestCarryingNoProof(t *testing.T) {
	// Nothing is stubbed on the market store: an unproven request must not reach the script or its candles.
	fixture := newIndicatorRouterUnderTest(t)

	request := httptest.NewRequest(http.MethodPost, "/indicator-calculations",
		strings.NewReader(indicatorBody))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
