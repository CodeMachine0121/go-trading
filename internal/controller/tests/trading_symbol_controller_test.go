package controller_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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

	tradingSymbolController := controller.NewTradingSymbolController(
		application.NewTradingSymbolApplication(
			service.NewTradingSymbolService(tradingSymbolRepository, kCandleRepository)))

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
			`[{"symbol":"BTCUSDT"},{"symbol":"ETHUSDT"},{"symbol":"XRPUSDT"}]`,
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
