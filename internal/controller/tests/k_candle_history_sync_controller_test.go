package controller_test

import (
	"context"
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
	engine                   *gin.Engine
	tradingSymbolRepository  *mocks.MockITradingSymbolRepository
	kCandleRepository        *mocks.MockIKCandleRepository
	marketDataProxy          *mocks.MockIMarketDataProxy
	historySyncRunRepository *mocks.MockIKCandleHistorySyncRunRepository
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
	historySyncRunRepository := mocks.NewMockIKCandleHistorySyncRunRepository(mockController)

	historySyncController := controller.NewKCandleHistorySyncController(
		application.NewKCandleIngestionApplication(service.NewKCandleIngestionService(
			kCandleRepository, historySyncRunRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
				vo.MarketCrypto: {},
			}), 5, time.Hour)),
		historySyncCeilingDays)

	engine := gin.New()
	engine.POST("/k-candles/history", historySyncController.StartSymbolHistorySync)
	engine.GET("/k-candles/history/:id", historySyncController.GetSymbolHistorySync)

	return historySyncRouterUnderTest{
		engine:                   engine,
		tradingSymbolRepository:  tradingSymbolRepository,
		kCandleRepository:        kCandleRepository,
		marketDataProxy:          marketDataProxy,
		historySyncRunRepository: historySyncRunRepository,
	}
}

func (underTest historySyncRouterUnderTest) get(path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	underTest.engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder
}

func (underTest historySyncRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost, "/k-candles/history", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	underTest.engine.ServeHTTP(recorder, request)

	return recorder
}

// acceptsAndFinishesEveryRun lets a sync be recorded and watched to its end, which is
// what the route hands back instead of candles.
func (underTest historySyncRouterUnderTest) acceptsAndFinishesEveryRun() chan struct{} {
	ended := make(chan struct{}, 1)
	underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleHistorySyncRun,
		) (entities.KCandleHistorySyncRun, error) {
			if syncRun.ID == 0 {
				syncRun.ID = 1
			}
			if vo.NewKCandleHistorySyncRunStatusVo(syncRun.Status) != vo.KCandleHistorySyncRunning {
				select {
				case ended <- struct{}{}:
				default:
				}
			}

			return syncRun, nil
		}).AnyTimes()

	return ended
}

// awaitRunEnding keeps a case from reading what the fetching did before it has done
// it. The fetching outlives the request, so the response arriving proves nothing
// about it.
func awaitRunEnding(t *testing.T, ended chan struct{}) {
	t.Helper()

	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("歷史同步沒有收尾")
	}
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
	ended := underTest.acceptsAndFinishesEveryRun()

	recorder := underTest.post(`{"symbol":"BTCUSDT","lookbackDays":30}`)

	// Accepted, not done. Years of candles is thousands of paced requests, so what
	// comes back is the run to watch rather than the work finished.
	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"status":"running"`)
	assert.Contains(t, recorder.Body.String(), `"totalChunks"`)
	awaitRunEnding(t, ended)
}

func TestLookingUpAHistorySyncAnswersWithWhereItGotTo(t *testing.T) {
	// The identifier the route hands back has to be worth something, or it is a
	// receipt for work nobody can see.
	underTest := newHistorySyncRouterUnderTest(t)
	finishedAt := time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	underTest.historySyncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
		entities.KCandleHistorySyncRun{
			ID: 7, Symbol: "BTCUSDT", LookbackDays: 30,
			Status:      string(vo.KCandleHistorySyncSucceeded),
			TotalChunks: 30, CompletedChunks: 30, StoredCount: 43200,
			StartedAt: time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), FinishedAt: &finishedAt,
		}, true, nil)

	recorder := underTest.get("/k-candles/history/7")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"completedChunks":30`)
	assert.Contains(t, recorder.Body.String(), `"storedCount":43200`)
}

func TestLookingUpAHistorySyncSaysWhenThereIsNoSuchRun(t *testing.T) {
	testCases := []struct {
		name               string
		path               string
		expectedStatusCode int
		arrange            func(underTest historySyncRouterUnderTest)
	}{
		{
			name:               "an identifier nobody started a run under",
			path:               "/k-candles/history/7",
			expectedStatusCode: http.StatusNotFound,
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.historySyncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
					Return(entities.KCandleHistorySyncRun{}, false, nil)
			},
		},
		{
			name:               "something that is not an identifier at all",
			path:               "/k-candles/history/abc",
			expectedStatusCode: http.StatusBadRequest,
			arrange:            func(historySyncRouterUnderTest) {},
		},
		{
			// This system failing to read its own storage is worth trying again for,
			// and must not read as "there is no such run".
			name:               "this system cannot read its own storage",
			path:               "/k-candles/history/7",
			expectedStatusCode: http.StatusBadGateway,
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.historySyncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
					Return(entities.KCandleHistorySyncRun{}, false, errors.New("storage unavailable"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newHistorySyncRouterUnderTest(t)
			testCase.arrange(underTest)

			recorder := underTest.get(testCase.path)

			assert.Equal(t, testCase.expectedStatusCode, recorder.Code)
		})
	}
}

func TestSyncingHistoryReachesBackAsFarAsAsked(t *testing.T) {
	// The figure in the body is the whole point of this route, so it has to arrive at
	// the window rather than being read as anything else.
	underTest := newHistorySyncRouterUnderTest(t)
	underTest.registersCrypto("BTCUSDT")
	firstAskedWindow := underTest.answersNothingForEveryChunk()
	ended := underTest.acceptsAndFinishesEveryRun()

	underTest.post(`{"symbol":"BTCUSDT","lookbackDays":3}`)

	awaitRunEnding(t, ended)
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
			// A source that would not answer is not this request's problem: the
			// request was fine, and what the source said lands on the run.
			name:               "a source that would not answer is still accepted",
			body:               `{"symbol":"BTCUSDT","lookbackDays":30}`,
			expectedStatusCode: http.StatusAccepted,
			expectedMessage:    "",
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.registersCrypto("BTCUSDT")
				underTest.acceptsAndFinishesEveryRun()
				underTest.marketDataProxy.EXPECT().
					FetchKCandles(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("market source unreachable")).AnyTimes()
			},
		},
		{
			// Asking twice is not a fault: the run already going is doing exactly
			// what the second one would, so it is refused rather than queued.
			name:               "a symbol that is already being fetched",
			body:               `{"symbol":"BTCUSDT","lookbackDays":30}`,
			expectedStatusCode: http.StatusConflict,
			expectedMessage:    "BTCUSDT",
			arrange: func(underTest historySyncRouterUnderTest) {
				underTest.tradingSymbolRepository.EXPECT().
					FindBySymbol(gomock.Any(), "BTCUSDT").
					Return(entities.TradingSymbol{
						Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
					}, true, nil)
				underTest.historySyncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.KCandleHistorySyncRun{},
						domains.KCandleHistorySyncInProgress("BTCUSDT"))
				underTest.marketDataProxy.EXPECT().
					FetchKCandles(gomock.Any(), gomock.Any()).Times(0)
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
	ended := underTest.acceptsAndFinishesEveryRun()

	underTest.post(`{"symbol":"BTCUSDT","lookbackDays":1,"interval":"1h"}`)

	awaitRunEnding(t, ended)
	// The window still ends on a whole minute: the extra field was not read at all.
	assert.Equal(t, 0, firstAskedWindow.EndTime.Second())
	assert.Equal(t, time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), firstAskedWindow.StartTime)
}
