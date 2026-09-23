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

func TestContractFundingRateApplicationCatchesContractsUpThroughToStorage(t *testing.T) {
	roundTime := time.Date(2026, 9, 23, 9, 30, 0, 0, time.UTC)
	settlement := vo.ContractFundingRateSettlementVo{
		Symbol:         "BTCUSDT",
		SettlementTime: time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC),
		FundingRate:    decimal.RequireFromString("0.0001"),
		MarkPrice:      decimal.NewNullDecimal(decimal.RequireFromString("87000")),
	}
	testCases := []struct {
		name string
		act  func(fundingRateApplication *application.ContractFundingRateApplication) (int, error)
	}{
		{name: "名單上每一檔", act: func(fundingRateApplication *application.ContractFundingRateApplication) (int, error) {
			report, roundError := fundingRateApplication.RunRound(t.Context())
			if roundError != nil {
				return 0, roundError
			}
			require.Len(t, report.SymbolReports, 1)

			return report.SymbolReports[0].StoredCount, nil
		}},
		{name: "剛加入名單的那一檔", act: func(fundingRateApplication *application.ContractFundingRateApplication) (int, error) {
			symbolReport, catchUpError := fundingRateApplication.CatchUpSymbol(t.Context(), "BTCUSDT")

			return symbolReport.StoredCount, catchUpError
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockController := gomock.NewController(t)
			settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
			symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
			fundingRateProxy := mocks.NewMockIContractFundingRateProxy(mockController)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(roundTime).AnyTimes()
			watched := entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}
			symbolRepository.EXPECT().FindWatched(gomock.Any()).Return([]entities.ContractTradingSymbol{watched}, nil).AnyTimes()
			symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(watched, true, nil).AnyTimes()
			settlementRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
				Return(entities.ContractFundingRateSettlement{}, false, nil)
			fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", time.Time{}).
				Return([]vo.ContractFundingRateSettlementVo{settlement}, nil)
			settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)
			fundingRateApplication := application.NewContractFundingRateApplication(
				service.NewContractFundingRateService(
					settlementRepository, symbolRepository, fundingRateProxy, clockProxy, 1000))

			storedCount, actError := testCase.act(fundingRateApplication)

			require.NoError(t, actError)
			assert.Equal(t, 1, storedCount)
		})
	}
}
