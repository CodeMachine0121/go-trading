package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type summaryServiceUnderTest struct {
	service                      *service.KCandleSummaryService
	kCandleRepository            *mocks.MockIKCandleRepository
	summaryDayRepository         *mocks.MockIKCandleSummaryDayRepository
	kCandleContractRepository    *mocks.MockIKCandleContractRepository
	contractSummaryDayRepository *mocks.MockIKCandleContractSummaryDayRepository
}

func newSummaryServiceUnderTest(
	t *testing.T, watchedSymbols []entities.TradingSymbol, watchedContracts []entities.ContractTradingSymbol,
) summaryServiceUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchedSymbols, nil)
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchedContracts, nil)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(summarizedSeriesNow).AnyTimes()
	underTest := summaryServiceUnderTest{
		kCandleRepository:            mocks.NewMockIKCandleRepository(mockController),
		summaryDayRepository:         mocks.NewMockIKCandleSummaryDayRepository(mockController),
		kCandleContractRepository:    mocks.NewMockIKCandleContractRepository(mockController),
		contractSummaryDayRepository: mocks.NewMockIKCandleContractSummaryDayRepository(mockController),
	}
	underTest.service = service.NewKCandleSummaryService(
		underTest.kCandleRepository, underTest.summaryDayRepository,
		underTest.kCandleContractRepository, underTest.contractSummaryDayRepository,
		tradingSymbolRepository, contractTradingSymbolRepository, clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}))

	return underTest
}

func summaryDayOn(day time.Time) gomock.Matcher {
	return gomock.Cond(func(summaryDay entities.KCandleSummaryDay) bool {
		return summaryDay.Day.Equal(day) && summaryDay.Summarized
	})
}

func TestBackgroundRoundSummarizesTheLastTwoFinishedDaysOfAWatchedSymbol(t *testing.T) {
	underTest := newSummaryServiceUnderTest(t,
		[]entities.TradingSymbol{{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}}, nil)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(16), octoberDay(18)).
		Return(nil, nil)
	gomock.InOrder(
		underTest.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), stretchOf(octoberDay(16), octoberDay(16).Add(lastNanosecondOfADay)), minutesInADay).
			Return(nil, nil),
		underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), summaryDayOn(octoberDay(16))).Return(true, nil),
		underTest.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), stretchOf(octoberDay(17), octoberDay(17).Add(lastNanosecondOfADay)), minutesInADay).
			Return(nil, nil),
		underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), summaryDayOn(octoberDay(17))).Return(true, nil),
	)

	require.NoError(t, underTest.service.SummarizeRecentDays(t.Context()))
}

func TestBackgroundRoundSkipsDaysAlreadySummarizedForAWatchedContract(t *testing.T) {
	underTest := newSummaryServiceUnderTest(t, nil,
		[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}})
	underTest.contractSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(16), octoberDay(18)).
		Return([]entities.KCandleContractSummaryDay{{Symbol: "BTCUSDT", Day: octoberDay(17), Revision: 1, Summarized: true}}, nil)
	underTest.kCandleContractRepository.EXPECT().
		FindInRange(gomock.Any(), stretchOf(octoberDay(16), octoberDay(16).Add(lastNanosecondOfADay)), minutesInADay).
		Return([]entities.KCandleContract{contractMinuteAt(octoberDay(16).Add(time.Hour), "100", "1")}, nil)
	underTest.contractSummaryDayRepository.EXPECT().
		SaveSummarized(gomock.Any(), gomock.Cond(func(summaryDay entities.KCandleContractSummaryDay) bool {
			return summaryDay.Day.Equal(octoberDay(16)) && len(summaryDay.Summaries) == 2
		})).
		Return(true, nil)

	require.NoError(t, underTest.service.SummarizeRecentDays(t.Context()))
}

func TestBackgroundRoundGoesOnPastASymbolThatFailsAndReportsIt(t *testing.T) {
	readFailure := errors.New("database unreachable")
	underTest := newSummaryServiceUnderTest(t,
		[]entities.TradingSymbol{{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}},
		[]entities.ContractTradingSymbol{{Symbol: "ETHUSDT", IsWatched: true}})
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(16), octoberDay(18)).
		Return(nil, readFailure)
	underTest.contractSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "ETHUSDT", vo.AggregationIntervalOneDay, octoberDay(16), octoberDay(18)).
		Return([]entities.KCandleContractSummaryDay{
			{Symbol: "ETHUSDT", Day: octoberDay(16), Summarized: true},
			{Symbol: "ETHUSDT", Day: octoberDay(17), Summarized: true},
		}, nil)

	roundError := underTest.service.SummarizeRecentDays(t.Context())

	assert.ErrorIs(t, roundError, readFailure)
	assert.ErrorContains(t, roundError, "BTCUSDT")
}

func TestBackgroundRoundReportsAnUnreadableWatchlist(t *testing.T) {
	spotWatchlistFailure := errors.New("spot watchlist unreadable")
	contractWatchlistFailure := errors.New("contract watchlist unreadable")
	testCases := []struct {
		name           string
		symbolsError   error
		contractsError error
		expectedError  error
	}{
		{"spot watchlist", spotWatchlistFailure, nil, spotWatchlistFailure},
		{"contract watchlist", nil, contractWatchlistFailure, contractWatchlistFailure},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockController := gomock.NewController(t)
			tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
			tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, testCase.symbolsError)
			contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
			contractTradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
				Return(nil, testCase.contractsError).MaxTimes(1)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(summarizedSeriesNow).AnyTimes()
			summaryService := service.NewKCandleSummaryService(
				mocks.NewMockIKCandleRepository(mockController), mocks.NewMockIKCandleSummaryDayRepository(mockController),
				mocks.NewMockIKCandleContractRepository(mockController), mocks.NewMockIKCandleContractSummaryDayRepository(mockController),
				tradingSymbolRepository, contractTradingSymbolRepository, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}))

			roundError := summaryService.SummarizeRecentDays(t.Context())

			assert.ErrorIs(t, roundError, testCase.expectedError)
		})
	}
}

func TestBackgroundRoundReportsAContractThatFails(t *testing.T) {
	readFailure := errors.New("database unreachable")
	underTest := newSummaryServiceUnderTest(t, nil,
		[]entities.ContractTradingSymbol{{Symbol: "ETHUSDT", IsWatched: true}})
	underTest.contractSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "ETHUSDT", vo.AggregationIntervalOneDay, octoberDay(16), octoberDay(18)).
		Return(nil, readFailure)

	roundError := underTest.service.SummarizeRecentDays(t.Context())

	assert.ErrorIs(t, roundError, readFailure)
	assert.ErrorContains(t, roundError, "ETHUSDT")
}
