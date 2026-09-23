package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestContractTradingSymbolApplicationRefreshesSpecificationsThroughToStorage(t *testing.T) {
	mockController := gomock.NewController(t)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	lookupProxy := mocks.NewMockIContractSymbolLookupProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).AnyTimes()
	symbolRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil)
	lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).Return([]vo.ContractTradingSpecificationVo{{
		Symbol: "BTCUSDT", TickSize: decimal.RequireFromString("0.1"), QuantityStep: decimal.RequireFromString("0.001"),
		MinimumQuantity: decimal.RequireFromString("0.001"), MinimumNotional: decimal.RequireFromString("50"),
		MaintenanceMarginRate: decimal.RequireFromString("0.025"), LiquidationFeeRate: decimal.RequireFromString("0.0125"),
	}}, nil)
	symbolRepository.EXPECT().SaveTradingSpecifications(gomock.Any(), gomock.Len(1)).Return(nil)
	symbolApplication := application.NewContractTradingSymbolApplication(
		service.NewContractTradingSymbolService(
			symbolRepository, mocks.NewMockIKCandleContractRepository(mockController), lookupProxy, clockProxy),
		nil, nil, nil)

	refreshedCount, refreshError := symbolApplication.RefreshTradingSpecifications(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 1, refreshedCount)
}
