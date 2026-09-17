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

// historySyncCeilingDays is the ceiling this router is built with. The refusal has to
// carry it, so a case can only check that by knowing what it is.
const historySyncCeilingDays = 90

type historySyncRouterUnderTest struct {
	engine                  *gin.Engine
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
	marketDataProxy         *mocks.MockIMarketDataProxy
}

// newHistorySyncRouterUnderTest wires the real application and domain service,
// mocking only the outermost boundaries: storage and the market source.
func newHistorySyncRouterUnderTest(t *testing.T) historySyncRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	historySyncController := controller.NewKCandleHistorySyncController(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
					vo.MarketCrypto: {},
				}), 5, time.Hour)),
		historySyncCeilingDays)

	engine := gin.New()
	engine.POST("/k-candles/history", historySyncController.SyncSymbolHistory)

	return historySyncRouterUnderTest{
		engine:                  engine,
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		marketDataProxy:         marketDataProxy,
	}
}

func (underTest historySyncRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost, "/k-candles/history", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	underTest.engine.ServeHTTP(recorder, request)

	return recorder
}

// registersCrypto is a symbol the system knows about, which is what makes a market —
// and therefore a source — available for it. Nothing is stored for it yet, so every
// chunk of the stretch is one this run has to go and ask about.
func (underTest historySyncRouterUnderTest) registersCrypto(symbol string) {
	underTest.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), symbol).
		Return(entities.TradingSymbol{
			Symbol: symbol, Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
}

// answersNothingForEveryChunk is a source that is reachable and simply has nothing to
// give. The stretch is walked a chunk at a time, so it is asked once per chunk.
func (underTest historySyncRouterUnderTest) answersNothingForEveryChunk() *vo.KCandleFetchWindowVo {
	firstAskedWindow := &vo.KCandleFetchWindowVo{}
	asked := false
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			if !asked {
				*firstAskedWindow = window
				asked = true
			}

			return []vo.MarketKCandleVo{}, nil
		}).AnyTimes()

	return firstAskedWindow
}

func TestSyncingHistoryReportsWhatItCollected(t *testing.T) {
	underTest := newHistorySyncRouterUnderTest(t)
	underTest.registersCrypto("BTCUSDT")
	underTest.answersNothingForEveryChunk()

	recorder := underTest.post(`{"symbol":"BTCUSDT","lookbackDays":30}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "symbolReports")
}

func TestSyncingHistoryReachesBackAsFarAsAsked(t *testing.T) {
	// The figure in the body is the whole point of this route, so it has to arrive at
	// the window rather than being read as anything else.
	underTest := newHistorySyncRouterUnderTest(t)
	underTest.registersCrypto("BTCUSDT")

	firstAskedWindow := underTest.answersNothingForEveryChunk()

	underTest.post(`{"symbol":"BTCUSDT","lookbackDays":3}`)

	assert.Equal(t, time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC), firstAskedWindow.StartTime)
}

func TestSyncingHistoryMapsEachRefusalOntoWhatTheCallerMustDoAboutIt(t *testing.T) {
	testCases := []struct {
		name               string
		body               string
		expectedStatusCode int
		expectedMessage    string
		arrange            func(underTest historySyncRouterUnderTest)
	}{
		{
			// Too far is the caller's to fix, and the answer has to say what to ask
			// for instead.
			name:               "reaching further back than allowed",
			body:               `{"symbol":"BTCUSDT","lookbackDays":91}`,
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "90",
			arrange:            func(historySyncRouterUnderTest) {},
		},
		{
			name:               "no days at all",
			body:               `{"symbol":"BTCUSDT","lookbackDays":0}`,
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "回溯天數",
			arrange:            func(historySyncRouterUnderTest) {},
		},
		{
			// Leaving it out reads as zero, and zero is refused — there is no default
			// standing in for the one figure this route exists to be told.
			name:               "not saying how far back",
			body:               `{"symbol":"BTCUSDT"}`,
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "回溯天數",
			arrange:            func(historySyncRouterUnderTest) {},
		},
		{
			name:               "a blank symbol is not a symbol",
			body:               `{"symbol":"   ","lookbackDays":30}`,
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "",
			arrange:            func(historySyncRouterUnderTest) {},
		},
		{
			// Registering it and retyping it are opposite instructions, so they must
			// not arrive as the same answer.
			name:               "a symbol nobody registered",
			body:               `{"symbol":"FOOBAR","lookbackDays":30}`,
			expectedStatusCode: http.StatusNotFound,
			expectedMessage:    "FOOBAR",
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.tradingSymbolRepository.EXPECT().
					FindBySymbol(gomock.Any(), "FOOBAR").
					Return(entities.TradingSymbol{}, false, nil)
			},
		},
		{
			// Anything this system broke on says "come back later", which is not the
			// same thing as "check what you asked for". A source that merely would not
			// answer is not among them: that lands in the report, because one symbol's
			// silence is a fact about the market rather than a failed request.
			name:               "this system could not read its own store",
			body:               `{"symbol":"BTCUSDT","lookbackDays":30}`,
			expectedStatusCode: http.StatusBadGateway,
			expectedMessage:    "storage unavailable",
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.tradingSymbolRepository.EXPECT().
					FindBySymbol(gomock.Any(), "BTCUSDT").
					Return(entities.TradingSymbol{}, false, errors.New("storage unavailable"))
			},
		},
		{
			// A source that would not answer is reported, not raised: the request did
			// what it was asked to do and found out something about the market.
			name:               "a source that would not answer is reported, not refused",
			body:               `{"symbol":"BTCUSDT","lookbackDays":30}`,
			expectedStatusCode: http.StatusOK,
			expectedMessage:    "fetchFailureReason",
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.registersCrypto("BTCUSDT")
				underTest.marketDataProxy.EXPECT().
					FetchKCandles(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("market source unreachable")).AnyTimes()
			},
		},
		{
			name:               "a body that cannot be read",
			body:               `{"symbol":`,
			expectedStatusCode: http.StatusBadRequest,
			expectedMessage:    "",
			arrange:            func(historySyncRouterUnderTest) {},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newHistorySyncRouterUnderTest(t)
			testCase.arrange(underTest)

			recorder := underTest.post(testCase.body)

			assert.Equal(t, testCase.expectedStatusCode, recorder.Code)
			if testCase.expectedMessage != "" {
				assert.Contains(t, recorder.Body.String(), testCase.expectedMessage)
			}
		})
	}
}

func TestSyncingHistoryOffersNoWayToChooseTheCoarseness(t *testing.T) {
	// The system stores one kind of candle and computes every coarser one from it.
	// A field for it would suggest there is another answer.
	underTest := newHistorySyncRouterUnderTest(t)
	underTest.registersCrypto("BTCUSDT")

	firstAskedWindow := underTest.answersNothingForEveryChunk()

	underTest.post(`{"symbol":"BTCUSDT","lookbackDays":1,"interval":"1h"}`)

	// The window still ends on a whole minute: the extra field was not read at all.
	assert.Equal(t, 0, firstAskedWindow.EndTime.Second())
	assert.Equal(t, time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), firstAskedWindow.StartTime)
}
