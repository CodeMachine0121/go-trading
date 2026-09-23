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

func marginTierRouterUnderTest(t *testing.T) (*gin.Engine, *mocks.MockIContractMaintenanceMarginTierRepository) {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	tierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(mockController)
	tierController := controller.NewContractMaintenanceMarginTierController(
		application.NewContractMaintenanceMarginTierApplication(service.NewContractMaintenanceMarginTierService(
			tierRepository, mocks.NewMockIContractTradingSymbolRepository(mockController),
			mocks.NewMockIContractMaintenanceMarginTierProxy(mockController), mocks.NewMockIClockProxy(mockController))))

	engine := gin.New()
	engine.GET("/contract-maintenance-margin-tiers", tierController.GetTiers)

	return engine, tierRepository
}

func TestContractMaintenanceMarginTierResponses(t *testing.T) {
	t.Run("reads a ladder first tier first", func(t *testing.T) {
		engine, tierRepository := marginTierRouterUnderTest(t)
		tierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			[]entities.ContractMaintenanceMarginTier{{
				Symbol: "BTCUSDT", Tier: 1, NotionalFloor: decimal.Zero, NotionalCap: decimal.RequireFromString("50000"),
				MaintenanceMarginRate: decimal.RequireFromString("0.004"), MaintenanceAmount: decimal.Zero,
				MaximumLeverage: 125, ConfirmedAt: at(8, 0),
			}}, nil)

		recorder := askFor(engine, "/contract-maintenance-margin-tiers?symbol=BTCUSDT")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[{"symbol":"BTCUSDT","tier":1,"notionalFloor":"0","notionalCap":"50000",
			"maintenanceMarginRate":"0.004","maintenanceAmount":"0","maximumLeverage":125,
			"confirmedAt":"2026-08-29T08:00:00Z"}]`, recorder.Body.String())
	})

	t.Run("answers a contract never fetched with an empty list", func(t *testing.T) {
		engine, tierRepository := marginTierRouterUnderTest(t)
		tierRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").Return([]entities.ContractMaintenanceMarginTier{}, nil)

		recorder := askFor(engine, "/contract-maintenance-margin-tiers?symbol=ETHUSDT")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.JSONEq(t, `[]`, recorder.Body.String())
	})

	t.Run("refuses a query without a contract", func(t *testing.T) {
		engine, _ := marginTierRouterUnderTest(t)

		recorder := askFor(engine, "/contract-maintenance-margin-tiers")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "必須指定交易標的")
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		engine, tierRepository := marginTierRouterUnderTest(t)
		tierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(nil, errors.New("storage unreachable"))

		recorder := askFor(engine, "/contract-maintenance-margin-tiers?symbol=BTCUSDT")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
