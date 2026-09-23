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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func fundingRateRouterUnderTest(t *testing.T) (*gin.Engine, *mocks.MockIContractFundingRateSettlementRepository) {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()
	settlementController := controller.NewContractFundingRateSettlementController(
		application.NewContractFundingRateApplication(service.NewContractFundingRateService(
			settlementRepository,
			mocks.NewMockIContractTradingSymbolRepository(mockController),
			mocks.NewMockIContractFundingRateProxy(mockController),
			clockProxy, queryMaxResults)))

	engine := gin.New()
	engine.GET("/contract-funding-rate-settlements", settlementController.GetSettlementsInRange)

	return engine, settlementRepository
}

func askFor(engine *gin.Engine, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))

	return recorder
}

func TestContractFundingRateSettlementResponses(t *testing.T) {
	t.Run("reads a range earliest first", func(t *testing.T) {
		engine, settlementRepository := fundingRateRouterUnderTest(t)
		settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(
			[]entities.ContractFundingRateSettlement{
				{Symbol: "BTCUSDT", SettlementTime: at(0, 0), FundingRate: decimal.RequireFromString("0.0001")},
				{Symbol: "BTCUSDT", SettlementTime: at(8, 0), FundingRate: decimal.RequireFromString("-0.00003"),
					MarkPrice: decimal.NewNullDecimal(decimal.RequireFromString("87000"))},
			}, nil)

		recorder := askFor(engine,
			"/contract-funding-rate-settlements?symbol=BTCUSDT&startTime=2026-08-29T00:00:00Z&endTime=2026-08-29T08:00:00Z")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[
			{"symbol":"BTCUSDT","settlementTime":"2026-08-29T00:00:00Z","fundingRate":"0.0001","markPrice":null},
			{"symbol":"BTCUSDT","settlementTime":"2026-08-29T08:00:00Z","fundingRate":"-0.00003","markPrice":"87000"}
		]`, recorder.Body.String())
	})

	t.Run("answers an empty stretch with an empty list", func(t *testing.T) {
		engine, settlementRepository := fundingRateRouterUnderTest(t)
		settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.ContractFundingRateSettlement{}, nil)

		recorder := askFor(engine,
			"/contract-funding-rate-settlements?symbol=BTCUSDT&startTime=2026-08-29T00:00:00Z&endTime=2026-08-29T08:00:00Z")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[]`, recorder.Body.String())
	})

	t.Run("refuses a query without a contract", func(t *testing.T) {
		engine, _ := fundingRateRouterUnderTest(t)

		recorder := askFor(engine,
			"/contract-funding-rate-settlements?startTime=2026-08-29T00:00:00Z&endTime=2026-08-29T08:00:00Z")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "必須指定交易標的")
	})

	t.Run("refuses a time it cannot read", func(t *testing.T) {
		engine, _ := fundingRateRouterUnderTest(t)

		startRecorder := askFor(engine,
			"/contract-funding-rate-settlements?symbol=BTCUSDT&startTime=yesterday&endTime=2026-08-29T08:00:00Z")
		endRecorder := askFor(engine,
			"/contract-funding-rate-settlements?symbol=BTCUSDT&startTime=2026-08-29T00:00:00Z&endTime=tomorrow")

		assert.Equal(t, http.StatusBadRequest, startRecorder.Code)
		assert.Contains(t, startRecorder.Body.String(), "startTime")
		assert.Equal(t, http.StatusBadRequest, endRecorder.Code)
		assert.Contains(t, endRecorder.Body.String(), "endTime")
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		engine, settlementRepository := fundingRateRouterUnderTest(t)
		settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("storage unreachable"))

		recorder := askFor(engine,
			"/contract-funding-rate-settlements?symbol=BTCUSDT&startTime=2026-08-29T00:00:00Z&endTime=2026-08-29T08:00:00Z")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
