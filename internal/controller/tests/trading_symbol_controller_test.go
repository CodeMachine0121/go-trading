package controller_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
}

func newTradingSymbolRouterUnderTest(t *testing.T) tradingSymbolRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)

	// Nothing is watched unless a test says so, so a listing that also asks what
	// holds a market's live places finds none held.
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil).AnyTimes()

	tradingSymbolController := controller.NewTradingSymbolController(
		application.NewTradingSymbolApplication(
			service.NewTradingSymbolService(
				tradingSymbolRepository, kCandleRepository,
				mocks.NewMockISymbolLookupProxy(mockController), tradingSymbolClockProxy(mockController),
				tradingSymbolMarketCatalog()),
			service.NewKCandleIngestionService(
				kCandleRepository, tradingSymbolRepository,
				mocks.NewMockIMarketDataProxy(mockController), tradingSymbolClockProxy(mockController),
				tradingSymbolMarketCatalog(), 5, time.Hour)))

	engine := gin.New()
	engine.GET("/trading-symbols", tradingSymbolController.ListTradingSymbols)

	return tradingSymbolRouterUnderTest{
		engine:                  engine,
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
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
			`[{"symbol":"BTCUSDT","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasLiveUpdates":true},{"symbol":"ETHUSDT","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasLiveUpdates":true},{"symbol":"XRPUSDT","market":"crypto","isWatched":false,"isWithinTradingSession":true,"hasLiveUpdates":true}]`,
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

// The handler must hand the application the request's own context rather than one it
// made up, and a caller that has already gone away is how that is told apart: the
// cancellation can only be there if it travelled the whole way from the request.
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

// tradingSymbolClockProxy stamps registrations with a moment the test states, rather
// than with whatever the wall clock said while it ran.
func tradingSymbolClockProxy(controller *gomock.Controller) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	return clockProxy
}

// tradingSymbolMarketCatalog is the markets these tests are written against.
func tradingSymbolMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
	})
}
