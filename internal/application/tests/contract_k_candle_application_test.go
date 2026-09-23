package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const contractQueryMaxResults = 1000

func contractAt(hour int, minute int) time.Time {
	return time.Date(2026, 8, 30, hour, minute, 0, 0, time.UTC)
}

func contractTradeCountOf(tradeCount int64) *int64 {
	return &tradeCount
}

func contractCandleWriteDto() dto.KCandleContractWriteDto {
	return dto.KCandleContractWriteDto{
		Symbol:              "BTCUSDT",
		OpenTime:            contractAt(9, 0),
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
		TradeCount:          contractTradeCountOf(7),
		MarkOpen:            decimal.NewNullDecimal(decimal.RequireFromString("101")),
		MarkHigh:            decimal.NewNullDecimal(decimal.RequireFromString("121")),
		MarkLow:             decimal.NewNullDecimal(decimal.RequireFromString("91")),
		MarkClose:           decimal.NewNullDecimal(decimal.RequireFromString("111")),
		IndexOpen:           decimal.NewNullDecimal(decimal.RequireFromString("102")),
		IndexHigh:           decimal.NewNullDecimal(decimal.RequireFromString("122")),
		IndexLow:            decimal.NewNullDecimal(decimal.RequireFromString("92")),
		IndexClose:          decimal.NewNullDecimal(decimal.RequireFromString("112")),
		PremiumIndexOpen:    decimal.NewNullDecimal(decimal.RequireFromString("-0.0001")),
		PremiumIndexHigh:    decimal.NewNullDecimal(decimal.RequireFromString("0.0002")),
		PremiumIndexLow:     decimal.NewNullDecimal(decimal.RequireFromString("-0.0003")),
		PremiumIndexClose:   decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
	}
}

type contractApplicationsUnderTest struct {
	candleApplication    *application.KCandleContractApplication
	symbolApplication    *application.ContractTradingSymbolApplication
	ingestionApplication *application.KCandleContractIngestionApplication
	candleRepository     *mocks.MockIKCandleContractRepository
	symbolRepository     *mocks.MockIContractTradingSymbolRepository
	syncRunRepository    *mocks.MockIKCandleContractHistorySyncRunRepository
	marketDataProxy      *mocks.MockIContractMarketDataProxy
	lookupProxy          *mocks.MockIContractSymbolLookupProxy
	settlementRepository *mocks.MockIContractFundingRateSettlementRepository
	fundingRateProxy     *mocks.MockIContractFundingRateProxy
	statisticRepository  *mocks.MockIContractPositionStatisticRepository
	statisticProxy       *mocks.MockIContractPositionStatisticProxy
}

func newContractApplicationsUnderTest(t *testing.T) contractApplicationsUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	syncRunRepository := mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	lookupProxy := mocks.NewMockIContractSymbolLookupProxy(mockController)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	fundingRateProxy := mocks.NewMockIContractFundingRateProxy(mockController)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	statisticProxy := mocks.NewMockIContractPositionStatisticProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(contractAt(9, 7)).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()

	ingestionService := service.NewContractKCandleIngestionService(
		candleRepository, syncRunRepository, symbolRepository, marketDataProxy, clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
		5, 24*time.Hour)

	return contractApplicationsUnderTest{
		candleApplication: application.NewKCandleContractApplication(
			service.NewKCandleContractService(candleRepository, clockProxy, contractQueryMaxResults)),
		symbolApplication: application.NewContractTradingSymbolApplication(
			service.NewContractTradingSymbolService(symbolRepository, candleRepository, lookupProxy, clockProxy),
			ingestionService,
			service.NewContractFundingRateService(
				settlementRepository, symbolRepository, fundingRateProxy, clockProxy, contractQueryMaxResults),
			service.NewContractPositionStatisticService(
				statisticRepository, symbolRepository, statisticProxy, clockProxy, contractQueryMaxResults)),
		ingestionApplication: application.NewKCandleContractIngestionApplication(ingestionService),
		candleRepository:     candleRepository,
		symbolRepository:     symbolRepository,
		syncRunRepository:    syncRunRepository,
		marketDataProxy:      marketDataProxy,
		lookupProxy:          lookupProxy,
		settlementRepository: settlementRepository,
		fundingRateProxy:     fundingRateProxy,
		statisticRepository:  statisticRepository,
		statisticProxy:       statisticProxy,
	}
}

func TestContractApplicationCarriesACandleThroughToStorage(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	underTest.candleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, storedCandle entities.KCandleContract) (entities.KCandleContract, error) {
			return storedCandle, nil
		})

	savedCandle, saveError := underTest.candleApplication.SaveKCandleContract(
		t.Context(), contractCandleWriteDto())

	require.NoError(t, saveError)
	assert.True(t, decimal.RequireFromString("111").Equal(savedCandle.MarkClose))
	assert.Equal(t, int64(7), savedCandle.TradeCount)
}

func TestContractApplicationReadsUpdatesAndDeletesOneCandle(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	storedCandle := entities.KCandleContract{Symbol: "BTCUSDT", OpenTime: contractAt(9, 0)}
	underTest.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandleContract{storedCandle}, nil)
	underTest.candleRepository.EXPECT().FindOne(gomock.Any(), "BTCUSDT", contractAt(9, 0)).
		Return(storedCandle, nil)
	underTest.candleRepository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(storedCandle, nil)
	underTest.candleRepository.EXPECT().Delete(gomock.Any(), "BTCUSDT", contractAt(9, 0)).Return(nil)

	rangeCandles, rangeError := underTest.candleApplication.GetKCandleContractsInRange(
		t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: contractAt(9, 0), EndTime: contractAt(9, 5),
		})
	readCandle, readError := underTest.candleApplication.GetKCandleContract(
		t.Context(), "BTCUSDT", contractAt(9, 0))
	_, updateError := underTest.candleApplication.UpdateKCandleContract(
		t.Context(), contractCandleWriteDto())
	deleteError := underTest.candleApplication.DeleteKCandleContract(
		t.Context(), "BTCUSDT", contractAt(9, 0))

	require.NoError(t, rangeError)
	require.NoError(t, readError)
	require.NoError(t, updateError)
	require.NoError(t, deleteError)
	assert.Len(t, rangeCandles, 1)
	assert.Equal(t, "BTCUSDT", readCandle.Symbol)
}

func TestContractApplicationCatchesAContractUpTheMomentItIsAdded(t *testing.T) {
	// Without this, a contract added now would hold nothing until the next start-up:
	// the scheduled round only collects what has closed since it last ran.
	underTest := newContractApplicationsUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
		Return(vo.ContractSymbolListingVo{IsListed: true}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: true,
	}).Return(nil)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil).Times(3)
	underTest.candleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandleContract{}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.ContractMarketKCandleVo{}, nil).Times(1)
	// Its whole funding rate history, from its first settlement.
	underTest.settlementRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
		Return(entities.ContractFundingRateSettlement{}, false, nil)
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", time.Time{}).
		Return([]vo.ContractFundingRateSettlementVo{}, nil).Times(1)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil)
	// And its last thirty days of position statistics.
	underTest.statisticRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
		Return(entities.ContractPositionStatistic{}, false, nil)
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(
		gomock.Any(), "BTCUSDT", contractAt(9, 5).Add(-30*24*time.Hour).Add(5*time.Minute), contractAt(9, 5)).
		Return([]vo.ContractPositionStatisticVo{}, nil).Times(1)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil)

	addError := underTest.symbolApplication.AddToWatchlist(t.Context(), "BTCUSDT")

	assert.NoError(t, addError)
}

func TestContractApplicationKeepsTheContractOnTheWatchlistWhenTheCatchUpFails(t *testing.T) {
	// The contract is on the watchlist, which is what was asked for and is true; the
	// ordinary rounds will reach it anyway.
	underTest := newContractApplicationsUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
		Return(vo.ContractSymbolListingVo{IsListed: true}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{}, false, nil).Times(3)

	addError := underTest.symbolApplication.AddToWatchlist(t.Context(), "BTCUSDT")

	assert.NoError(t, addError)
}

func TestContractApplicationDoesNotCatchUpAContractItRefusedToAdd(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "NOSUCHPAIR").
		Return(vo.ContractSymbolListingVo{}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Times(0)

	addError := underTest.symbolApplication.AddToWatchlist(t.Context(), "NOSUCHPAIR")

	assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
}

func TestContractApplicationListsAndStopsFollowingContracts(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil)
	underTest.candleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: false,
	}).Return(nil)

	contractSymbols, listError := underTest.symbolApplication.ListContractTradingSymbols(t.Context())
	removeError := underTest.symbolApplication.RemoveFromWatchlist(t.Context(), "BTCUSDT")

	require.NoError(t, listError)
	require.NoError(t, removeError)
	require.Len(t, contractSymbols, 1)
	assert.True(t, contractSymbols[0].IsWatched)
}

func TestContractIngestionApplicationDrivesEveryWayOfFetching(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil).AnyTimes()
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil).AnyTimes()
	underTest.candleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandleContract{}, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return([]vo.ContractMarketKCandleVo{}, nil).AnyTimes()
	underTest.syncRunRepository.EXPECT().FailAllRunning(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(3, nil)

	roundReport, roundError := underTest.ingestionApplication.RunScheduledRound(t.Context())
	backfillReport, backfillError := underTest.ingestionApplication.RunBackfill(t.Context())
	catchUpReport, catchUpError := underTest.ingestionApplication.CatchUpSymbol(t.Context(), "BTCUSDT")
	sweptCount, sweepError := underTest.ingestionApplication.FailInterruptedHistorySyncs(t.Context())

	require.NoError(t, roundError)
	require.NoError(t, backfillError)
	require.NoError(t, catchUpError)
	require.NoError(t, sweepError)
	assert.Len(t, roundReport.SymbolReports, 1)
	assert.Len(t, backfillReport.SymbolReports, 1)
	assert.Len(t, catchUpReport.SymbolReports, 1)
	assert.Equal(t, 3, sweptCount)
}

func TestContractIngestionApplicationStartsAndReadsAHistorySync(t *testing.T) {
	underTest := newContractApplicationsUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ any, syncRun entities.KCandleContractHistorySyncRun,
		) (entities.KCandleContractHistorySyncRun, error) {
			syncRun.ID = 11

			return syncRun, nil
		}).AnyTimes()
	underTest.candleRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(100000, nil).AnyTimes()
	underTest.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(11)).Return(
		entities.KCandleContractHistorySyncRun{
			ID: 11, Symbol: "BTCUSDT", Status: string(vo.KCandleHistorySyncSucceeded),
			StartedAt: contractAt(9, 0),
		}, true, nil)

	startedRun, startError := underTest.ingestionApplication.StartSymbolHistorySync(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: 1}, 3650)
	readRun, readError := underTest.ingestionApplication.GetSymbolHistorySync(t.Context(), 11)

	require.NoError(t, startError)
	require.NoError(t, readError)
	assert.Equal(t, uint(11), startedRun.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncRunning), startedRun.Status)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), readRun.Status)
}
