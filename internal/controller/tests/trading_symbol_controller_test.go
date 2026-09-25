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

type tradingSymbolRouterUnderTest struct {
	engine                  *gin.Engine
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
	symbolLookupProxy       *mocks.MockISymbolLookupProxy
	marketDataProxy         *mocks.MockIMarketDataProxy
}

func newTradingSymbolRouterUnderTest(t *testing.T) tradingSymbolRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	symbolLookupProxy := mocks.NewMockISymbolLookupProxy(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)

	// Nothing is watched unless a test says so.
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil).AnyTimes()

	tradingSymbolController := controller.NewTradingSymbolController(
		application.NewTradingSymbolApplication(
			service.NewTradingSymbolService(
				tradingSymbolRepository, kCandleRepository,
				symbolLookupProxy, tradingSymbolClockProxy(mockController),
				tradingSymbolMarketCatalog()), service.NewKCandleIngestionService(
				kCandleRepository, mocks.NewMockIKCandleHistorySyncRunRepository(mockController), tradingSymbolRepository,
				marketDataProxy, tradingSymbolClockProxy(mockController),
				tradingSymbolMarketCatalog(), 5, time.Hour)))

	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.GET("/trading-symbols", tradingSymbolController.ListTradingSymbols)
	engine.POST("/watchlist", requiresSignIn, tradingSymbolController.AddToWatchlist)
	engine.DELETE("/watchlist/:symbol", requiresSignIn, tradingSymbolController.RemoveFromWatchlist)

	return tradingSymbolRouterUnderTest{
		engine:                  engine,
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		symbolLookupProxy:       symbolLookupProxy,
		marketDataProxy:         marketDataProxy,
	}
}

func (fixture tradingSymbolRouterUnderTest) get() *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/trading-symbols", nil)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func TestListTradingSymbolsResponses(t *testing.T) {
	t.Run("returns the registered markets together with the ones holding candles", func(t *testing.T) {
		fixture := newTradingSymbolRouterUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindAll(gomock.Any()).Return([]entities.TradingSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}, nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{"XRPUSDT"}, nil)

		recorder := fixture.get()

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t,
			`[{"symbol":"BTCUSDT","displayName":"","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasTradingSession":false,"hasLiveUpdates":true},{"symbol":"ETHUSDT","displayName":"","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasTradingSession":false,"hasLiveUpdates":true},{"symbol":"XRPUSDT","displayName":"","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasTradingSession":false,"hasLiveUpdates":true}]`,
			recorder.Body.String())
	})

	t.Run("returns an empty list when the system knows of no market at all", func(t *testing.T) {
		fixture := newTradingSymbolRouterUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{}, nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

		recorder := fixture.get()

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "[]", recorder.Body.String())
	})

	t.Run("reports a storage failure as a bad gateway", func(t *testing.T) {
		fixture := newTradingSymbolRouterUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, errors.New("storage unreachable"))

		recorder := fixture.get()

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

// Only a context cancelled at the request can prove the handler passes the request's own context through.
func TestTheRequestsOwnContextReachesStorage(t *testing.T) {
	fixture := newTradingSymbolRouterUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		DoAndReturn(func(executionContext context.Context) ([]entities.TradingSymbol, error) {
			assert.ErrorIs(t, executionContext.Err(), context.Canceled)
			return []entities.TradingSymbol{}, nil
		})
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).
		Return([]string{}, nil).AnyTimes()

	callerWentAway, abandonTheRequest := context.WithCancel(t.Context())
	abandonTheRequest()
	request := httptest.NewRequest(http.MethodGet, "/trading-symbols", nil).WithContext(callerWentAway)

	fixture.engine.ServeHTTP(httptest.NewRecorder(), request)
}

// tradingSymbolClockProxy stamps registrations with a fixed moment.
func tradingSymbolClockProxy(controller *gomock.Controller) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	return clockProxy
}

func tradingSymbolMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
	})
}

func TestChangingTheWatchlistRefusesAVisitor(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "adding", method: http.MethodPost, target: "/watchlist", body: `{"symbol":"ETHUSDT","market":"crypto"}`},
		{name: "removing", method: http.MethodDelete, target: "/watchlist/ETHUSDT"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// No lookup or save expectation is set, so the watchlist cannot change unnoticed.
			fixture := newTradingSymbolRouterUnderTest(t)

			recorder := requestWithoutProof(fixture.engine, testCase.method, testCase.target, testCase.body)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestAnActivatedUserRemovesASymbolFromTheWatchlist(t *testing.T) {
	fixture := newTradingSymbolRouterUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").
		Return(entities.TradingSymbol{Symbol: "ETHUSDT", Market: string(vo.MarketCrypto), IsWatched: true}, true, nil)
	fixture.tradingSymbolRepository.EXPECT().
		Save(gomock.Any(), entities.TradingSymbol{Symbol: "ETHUSDT", Market: string(vo.MarketCrypto), IsWatched: false}).
		Return(nil)
	request := httptest.NewRequest(http.MethodDelete, "/watchlist/ETHUSDT", nil)
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()

	fixture.engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAnActivatedUserAddsASymbolToTheWatchlist(t *testing.T) {
	fixture := newTradingSymbolRouterUnderTest(t)
	fixture.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), vo.MarketCrypto, "ETHUSDT").
		Return(vo.SymbolListingVo{IsListed: true}, nil)
	fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").
		Return(entities.TradingSymbol{Symbol: "ETHUSDT", Market: string(vo.MarketCrypto), IsWatched: true}, true, nil).
		AnyTimes()
	fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, savedSymbol entities.TradingSymbol) error {
			assert.True(t, savedSymbol.IsWatched)
			return nil
		})
	// Joining the watchlist catches the symbol up; the source has nothing new.
	fixture.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "ETHUSDT", 1).Return([]entities.KCandle{}, nil)
	fixture.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return([]vo.MarketKCandleVo{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/watchlist", strings.NewReader(`{"symbol":"ETHUSDT","market":"crypto"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()

	fixture.engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}
