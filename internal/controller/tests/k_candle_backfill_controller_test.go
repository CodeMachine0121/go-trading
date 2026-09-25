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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type backfillRouterUnderTest struct {
	engine                  *gin.Engine
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
	marketDataProxy         *mocks.MockIMarketDataProxy
}

// newBackfillRouterUnderTest wires real application and domain services, mocking only storage and the market source.
func newBackfillRouterUnderTest(t *testing.T) backfillRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	backfillController := controller.NewKCandleBackfillController(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, mocks.NewMockIKCandleHistorySyncRunRepository(mockController), tradingSymbolRepository, marketDataProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
					vo.MarketCrypto: {},
				}), 5, time.Hour)))

	engine := gin.New()
	engine.POST("/k-candles/backfill", backfillController.CatchUpSymbol)

	return backfillRouterUnderTest{
		engine:                  engine,
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		marketDataProxy:         marketDataProxy,
	}
}

func (underTest backfillRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost, "/k-candles/backfill", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	underTest.engine.ServeHTTP(recorder, request)

	return recorder
}

func TestCatchingUpASymbolReportsWhatItCollected(t *testing.T) {
	underTest := newBackfillRouterUnderTest(t)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.MarketKCandleVo{}, nil)

	response := underTest.post(`{"symbol":"BTCUSDT"}`)

	// The wire field names are the contract, so the whole body is pinned.
	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"symbolReports":[{"symbol":"BTCUSDT","market":"crypto","wasAsked":true,"storedCount":0,"skippedCount":0,"skippedKCandles":[],"skippedKCandlesTruncated":false,"fetchFailureReason":""}]}`, response.Body.String())
}

func TestCatchingUpAnswersWithASkippedListEvenWhenTheSourceWillNotAnswer(t *testing.T) {
	// The skipped list must be [] rather than null on this path, where callers actually count it.
	underTest := newBackfillRouterUnderTest(t)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("source unavailable"))

	response := underTest.post(`{"symbol":"BTCUSDT"}`)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"symbolReports":[{"symbol":"BTCUSDT","market":"crypto","wasAsked":false,"storedCount":0,"skippedCount":0,"skippedKCandles":[],"skippedKCandlesTruncated":false,"fetchFailureReason":"source unavailable"}]}`, response.Body.String())
}

func TestCatchingUpASymbolNobodyRegisteredIsAnsweredAsNotFound(t *testing.T) {
	// An unknown symbol is the caller's to fix, so it must not look like an unavailable source.
	underTest := newBackfillRouterUnderTest(t)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "9999").
		Return(entities.TradingSymbol{}, false, nil)

	response := underTest.post(`{"symbol":"9999"}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestCatchingUpWithNoSymbolNamedIsRefusedAsTheCallersMistake(t *testing.T) {
	underTest := newBackfillRouterUnderTest(t)

	response := underTest.post(`{"symbol":"  "}`)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestCatchingUpIsAnsweredAsThisSystemsFaultWhenStorageWillNotAnswer(t *testing.T) {
	// The request is valid and worth retrying, so it must not be answered as a client error.
	underTest := newBackfillRouterUnderTest(t)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.TradingSymbol{}, false, errors.New("storage unreachable"))

	response := underTest.post(`{"symbol":"BTCUSDT"}`)

	assert.Equal(t, http.StatusBadGateway, response.Code)
}
