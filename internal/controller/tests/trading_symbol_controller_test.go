package controller_test

import (
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
			FindAll().Return([]entities.TradingSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}, nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols().Return([]string{"XRPUSDT"}, nil)

		recorder := fixture.get()

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t,
			`[{"symbol":"BTCUSDT"},{"symbol":"ETHUSDT"},{"symbol":"XRPUSDT"}]`,
			recorder.Body.String())
	})

	t.Run("returns an empty list when the system knows of no market at all", func(t *testing.T) {
		fixture := newTradingSymbolRouterUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll().Return([]entities.TradingSymbol{}, nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols().Return([]string{}, nil)

		recorder := fixture.get()

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "[]", recorder.Body.String())
	})

	t.Run("reports a storage failure as a bad gateway", func(t *testing.T) {
		fixture := newTradingSymbolRouterUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll().Return(nil, errors.New("storage unreachable"))

		recorder := fixture.get()

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
