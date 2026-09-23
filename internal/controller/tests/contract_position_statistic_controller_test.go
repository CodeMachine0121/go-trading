package controller_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func positionStatisticRouterUnderTest(t *testing.T) (*gin.Engine, *mocks.MockIContractPositionStatisticRepository) {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()
	statisticController := controller.NewContractPositionStatisticController(
		application.NewContractPositionStatisticApplication(service.NewContractPositionStatisticService(
			statisticRepository,
			mocks.NewMockIContractTradingSymbolRepository(mockController),
			mocks.NewMockIContractPositionStatisticProxy(mockController),
			clockProxy, queryMaxResults)))

	engine := gin.New()
	engine.GET("/contract-position-statistics", statisticController.GetStatisticsInRange)

	return engine, statisticRepository
}

func TestContractPositionStatisticResponses(t *testing.T) {
	const aStretch = "&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:05:00Z"

	t.Run("reads a range earliest first", func(t *testing.T) {
		engine, statisticRepository := positionStatisticRouterUnderTest(t)
		statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(
			[]entities.ContractPositionStatistic{{
				Symbol: "BTCUSDT", StatisticTime: at(9, 0),
				OpenInterest: decimal.RequireFromString("106479.865"), OpenInterestValue: decimal.RequireFromString("9262820059.48"),
				AccountLongShare: decimal.RequireFromString("0.47"), AccountShortShare: decimal.RequireFromString("0.53"),
				AccountLongShortRatio: decimal.RequireFromString("0.89"), TopTraderPositionLongShare: decimal.RequireFromString("0.6688"),
				TopTraderPositionShortShare: decimal.RequireFromString("0.3312"), TopTraderPositionLongShortRatio: decimal.RequireFromString("2.0189"),
			}}, nil)

		recorder := askFor(engine, "/contract-position-statistics?symbol=BTCUSDT"+aStretch)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[{"symbol":"BTCUSDT","statisticTime":"2026-08-29T09:00:00Z",
			"openInterest":"106479.865","openInterestValue":"9262820059.48",
			"accountLongShare":"0.47","accountShortShare":"0.53","accountLongShortRatio":"0.89",
			"topTraderPositionLongShare":"0.6688","topTraderPositionShortShare":"0.3312",
			"topTraderPositionLongShortRatio":"2.0189"}]`, recorder.Body.String())
	})

	t.Run("answers an empty stretch with an empty list", func(t *testing.T) {
		engine, statisticRepository := positionStatisticRouterUnderTest(t)
		statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.ContractPositionStatistic{}, nil)

		recorder := askFor(engine, "/contract-position-statistics?symbol=BTCUSDT"+aStretch)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[]`, recorder.Body.String())
	})

	t.Run("refuses a query without a contract", func(t *testing.T) {
		engine, _ := positionStatisticRouterUnderTest(t)

		recorder := askFor(engine, "/contract-position-statistics?"+aStretch[1:])

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "必須指定交易標的")
	})

	t.Run("refuses a time it cannot read", func(t *testing.T) {
		engine, _ := positionStatisticRouterUnderTest(t)

		startRecorder := askFor(engine, "/contract-position-statistics?symbol=BTCUSDT&startTime=x&endTime=2026-08-29T09:05:00Z")
		endRecorder := askFor(engine, "/contract-position-statistics?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=x")

		assert.Equal(t, http.StatusBadRequest, startRecorder.Code)
		assert.Contains(t, startRecorder.Body.String(), "startTime")
		assert.Equal(t, http.StatusBadRequest, endRecorder.Code)
		assert.Contains(t, endRecorder.Body.String(), "endTime")
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		engine, statisticRepository := positionStatisticRouterUnderTest(t)
		statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("storage unreachable"))

		recorder := askFor(engine, "/contract-position-statistics?symbol=BTCUSDT"+aStretch)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
