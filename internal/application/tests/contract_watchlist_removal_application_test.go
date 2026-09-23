package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRemovingAContractOnlyStopsFollowingIt(t *testing.T) {
	// After it leaves the watchlist, neither the funding rate round nor the position
	// statistic round asks about it — and everything already held for it is still
	// there to be read. The two proxies below carry no expectation at all: any
	// question put to the venue fails the test.
	mockController := gomock.NewController(t)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)).AnyTimes()
	fundingRateService := service.NewContractFundingRateService(settlementRepository, symbolRepository,
		mocks.NewMockIContractFundingRateProxy(mockController), clockProxy, 1000)
	positionStatisticService := service.NewContractPositionStatisticService(statisticRepository, symbolRepository,
		mocks.NewMockIContractPositionStatisticProxy(mockController), clockProxy, 1000)
	symbolApplication := application.NewContractTradingSymbolApplication(
		service.NewContractTradingSymbolService(symbolRepository,
			mocks.NewMockIKCandleContractRepository(mockController),
			mocks.NewMockIContractSymbolLookupProxy(mockController), clockProxy),
		nil, fundingRateService, positionStatisticService)
	symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: false}).
		Return(nil)
	// What the watchlist holds once BTCUSDT has left it.
	symbolRepository.EXPECT().FindWatched(gomock.Any()).Return([]entities.ContractTradingSymbol{}, nil).Times(2)
	settledAt := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		[]entities.ContractFundingRateSettlement{
			{Symbol: "BTCUSDT", SettlementTime: settledAt, FundingRate: decimal.RequireFromString("0.0001")},
		}, nil)
	statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		[]entities.ContractPositionStatistic{{Symbol: "BTCUSDT", StatisticTime: settledAt}}, nil)

	removeError := symbolApplication.RemoveFromWatchlist(t.Context(), "BTCUSDT")
	fundingRateReport, fundingRateError := application.NewContractFundingRateApplication(fundingRateService).
		RunRound(t.Context())
	positionStatisticReport, positionStatisticError := application.
		NewContractPositionStatisticApplication(positionStatisticService).RunRound(t.Context())
	aDay := dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: settledAt.Add(-time.Hour), EndTime: settledAt.Add(time.Hour)}
	heldSettlements, settlementsError := application.NewContractFundingRateApplication(fundingRateService).
		GetSettlementsInRange(t.Context(), aDay)
	heldStatistics, statisticsError := application.NewContractPositionStatisticApplication(positionStatisticService).
		GetStatisticsInRange(t.Context(), aDay)

	require.NoError(t, removeError)
	require.NoError(t, fundingRateError)
	require.NoError(t, positionStatisticError)
	assert.Empty(t, fundingRateReport.SymbolReports)
	assert.Empty(t, positionStatisticReport.SymbolReports)
	require.NoError(t, settlementsError)
	require.Len(t, heldSettlements, 1)
	assert.Equal(t, settledAt, heldSettlements[0].SettlementTime)
	require.NoError(t, statisticsError)
	require.Len(t, heldStatistics, 1)
	assert.Equal(t, settledAt, heldStatistics[0].StatisticTime)
}
