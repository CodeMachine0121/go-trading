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

// newBackfillRouterUnderTest wires the real application and domain service, mocking
// only the outermost boundaries: storage and the market source.
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
				kCandleRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
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

	// The names on the wire are the contract, so the whole body is pinned rather than
	// only searched for the symbol. A caller looking for storedCount and handed
	// StoredCount reads nothing at all, and finds out only when it goes to add the
	// counts up — a page-breaking error about a field, one layer away from the field.
	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"symbolReports":[{"symbol":"BTCUSDT","market":"crypto","wasAsked":true,"storedCount":0,"skippedKCandles":[],"fetchFailureReason":""}]}`, response.Body.String())
}

func TestCatchingUpAnswersWithASkippedListEvenWhenTheSourceWillNotAnswer(t *testing.T) {
	// The path a reader inspects the skipped list on is this one, not the happy one.
	// Answered with null here, a caller counting the list would break on exactly the
	// round it was asking about — and nowhere else, so nobody would find it.
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
	assert.JSONEq(t, `{"symbolReports":[{"symbol":"BTCUSDT","market":"crypto","wasAsked":false,"storedCount":0,"skippedKCandles":[],"fetchFailureReason":"source unavailable"}]}`, response.Body.String())
}

func TestCatchingUpASymbolNobodyRegisteredIsAnsweredAsNotFound(t *testing.T) {
	// "We have never heard of this" is the caller's to fix. Answering it the same way
	// as a source that would not answer sends them off to wait for something that is
	// never going to happen.
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
	// Nothing is wrong with the request, so it must not be answered as though there
	// were: this one is worth making again, and the one above is not.
	underTest := newBackfillRouterUnderTest(t)
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.TradingSymbol{}, false, errors.New("storage unreachable"))

	response := underTest.post(`{"symbol":"BTCUSDT"}`)

	assert.Equal(t, http.StatusBadGateway, response.Code)
}
