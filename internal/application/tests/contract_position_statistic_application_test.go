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

func TestContractPositionStatisticApplicationRecordsContractsThroughToStorage(t *testing.T) {
	roundTime := time.Date(2026, 9, 23, 10, 0, 30, 0, time.UTC)
	share := func(value string) decimal.NullDecimal {
		return decimal.NewNullDecimal(decimal.RequireFromString(value))
	}
	statistic := vo.ContractPositionStatisticVo{
		Symbol: "BTCUSDT", StatisticTime: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC),
		OpenInterest: decimal.RequireFromString("100"), OpenInterestValue: decimal.RequireFromString("8700000"),
		AccountLongShare: share("0.5"), AccountShortShare: share("0.5"), AccountLongShortRatio: share("1"),
		TopTraderPositionLongShare: share("0.5"), TopTraderPositionShortShare: share("0.5"),
		TopTraderPositionLongShortRatio: share("1"),
	}
	testCases := []struct {
		name string
		act  func(statisticApplication *application.ContractPositionStatisticApplication) (int, error)
	}{
		{name: "名單上每一檔", act: func(statisticApplication *application.ContractPositionStatisticApplication) (int, error) {
			report, roundError := statisticApplication.RunRound(t.Context())
			if roundError != nil {
				return 0, roundError
			}
			require.Len(t, report.SymbolReports, 1)

			return report.SymbolReports[0].StoredCount, nil
		}},
		{name: "剛加入名單的那一檔", act: func(statisticApplication *application.ContractPositionStatisticApplication) (int, error) {
			symbolReport, catchUpError := statisticApplication.CatchUpSymbol(t.Context(), "BTCUSDT")

			return symbolReport.StoredCount, catchUpError
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockController := gomock.NewController(t)
			statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
			symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
			statisticProxy := mocks.NewMockIContractPositionStatisticProxy(mockController)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(roundTime).AnyTimes()
			watched := entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}
			symbolRepository.EXPECT().FindWatched(gomock.Any()).Return([]entities.ContractTradingSymbol{watched}, nil).AnyTimes()
			symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(watched, true, nil).AnyTimes()
			statisticRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
				Return(entities.ContractPositionStatistic{}, false, nil)
			statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
				Return([]vo.ContractPositionStatisticVo{statistic}, nil)
			statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)
			statisticApplication := application.NewContractPositionStatisticApplication(
				service.NewContractPositionStatisticService(
					statisticRepository, symbolRepository, statisticProxy,
					mocks.NewMockIContractPositionStatisticArchiveProxy(mockController), clockProxy, 1000))

			storedCount, actError := testCase.act(statisticApplication)

			require.NoError(t, actError)
			assert.Equal(t, 1, storedCount)
		})
	}
}
