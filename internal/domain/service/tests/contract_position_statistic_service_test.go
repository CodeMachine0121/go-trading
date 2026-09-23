package service_test

import (
	"errors"
	"testing"
	"time"

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

var statisticRoundTime = time.Date(2026, 9, 23, 10, 0, 30, 0, time.UTC)

func statisticMoment(hour int, minute int) time.Time {
	return time.Date(2026, 9, 23, hour, minute, 0, 0, time.UTC)
}

func present(value string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(value))
}

func venueStatistic(symbol string, statisticTime time.Time) vo.ContractPositionStatisticVo {
	return vo.ContractPositionStatisticVo{
		Symbol:                          symbol,
		StatisticTime:                   statisticTime,
		OpenInterest:                    decimal.RequireFromString("100"),
		OpenInterestValue:               decimal.RequireFromString("8700000"),
		AccountLongShare:                present("0.47"),
		AccountShortShare:               present("0.53"),
		AccountLongShortRatio:           present("0.89"),
		TopTraderPositionLongShare:      present("0.6"),
		TopTraderPositionShortShare:     present("0.4"),
		TopTraderPositionLongShortRatio: present("1.5"),
	}
}

type positionStatisticServiceUnderTest struct {
	service             *service.ContractPositionStatisticService
	statisticRepository *mocks.MockIContractPositionStatisticRepository
	symbolRepository    *mocks.MockIContractTradingSymbolRepository
	statisticProxy      *mocks.MockIContractPositionStatisticProxy
}

func newPositionStatisticServiceUnderTest(t *testing.T) positionStatisticServiceUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	statisticProxy := mocks.NewMockIContractPositionStatisticProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(statisticRoundTime).AnyTimes()

	return positionStatisticServiceUnderTest{
		service: service.NewContractPositionStatisticService(
			statisticRepository, symbolRepository, statisticProxy, clockProxy, 3),
		statisticRepository: statisticRepository,
		symbolRepository:    symbolRepository,
		statisticProxy:      statisticProxy,
	}
}

func (underTest positionStatisticServiceUnderTest) watching(symbols ...string) {
	watchedSymbols := make([]entities.ContractTradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true})
	}
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchedSymbols, nil)
}

func (underTest positionStatisticServiceUnderTest) latestHeld(symbol string, statisticTime time.Time) {
	underTest.statisticRepository.EXPECT().FindLatest(gomock.Any(), symbol).Return(
		entities.ContractPositionStatistic{Symbol: symbol, StatisticTime: statisticTime}, true, nil)
}

func TestPositionStatisticRoundAsksFromAnHourBehindTheLastHeldStatisticToNow(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", statisticMoment(9, 5))
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(
		gomock.Any(), "BTCUSDT", statisticMoment(8, 10), statisticMoment(10, 0)).
		Return([]vo.ContractPositionStatisticVo{venueStatistic("BTCUSDT", statisticMoment(9, 10))}, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
}

func TestPositionStatisticRoundStartsThirtyDaysBackForAContractNeverRecorded(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.statisticRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
		Return(entities.ContractPositionStatistic{}, false, nil)
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(
		gomock.Any(), "BTCUSDT", time.Date(2026, 8, 24, 10, 5, 0, 0, time.UTC), statisticMoment(10, 0)).
		Return(nil, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(0)).Return(0, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
}

func TestPositionStatisticRoundAsksNothingWhenNothingCanBeAskedAboutYet(t *testing.T) {
	// Only a latest statistic later than now — a clock set back — leaves no stretch.
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", statisticMoment(12, 0))

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
}

func TestPositionStatisticRoundSkipsTheMomentMissingASplitAndStoresTheRest(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", statisticMoment(9, 0))
	withoutTopTraderSplit := venueStatistic("BTCUSDT", statisticMoment(9, 5))
	withoutTopTraderSplit.TopTraderPositionLongShare = decimal.NullDecimal{}
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]vo.ContractPositionStatisticVo{withoutTopTraderSplit, venueStatistic("BTCUSDT", statisticMoment(9, 10))}, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	symbolReport := report.SymbolReports[0]
	assert.Equal(t, 1, symbolReport.StoredCount)
	require.Len(t, symbolReport.SkippedRecords, 1)
	assert.Equal(t, statisticMoment(9, 5), symbolReport.SkippedRecords[0].RecordTime)
	assert.Contains(t, symbolReport.SkippedRecords[0].Reason, "缺大戶多空持倉比")
}

func TestPositionStatisticRoundLeavesOneContractsFailureToItself(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT", "ETHUSDT")
	underTest.latestHeld("BTCUSDT", statisticMoment(9, 55))
	underTest.latestHeld("ETHUSDT", statisticMoment(9, 55))
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(nil, errors.New("venue unreachable"))
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), "ETHUSDT", gomock.Any(), gomock.Any()).
		Return([]vo.ContractPositionStatisticVo{venueStatistic("ETHUSDT", statisticMoment(10, 0))}, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	require.Len(t, report.SymbolReports, 2)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "venue unreachable")
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Equal(t, 1, report.SymbolReports[1].StoredCount)
}

func TestPositionStatisticRoundReportsStorageFailingForThatContract(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest positionStatisticServiceUnderTest)
	}{
		{name: "讀不到上一次存到哪", arrange: func(underTest positionStatisticServiceUnderTest) {
			underTest.statisticRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
				Return(entities.ContractPositionStatistic{}, false, errors.New("storage unreachable"))
		}},
		{name: "存不進去", arrange: func(underTest positionStatisticServiceUnderTest) {
			underTest.latestHeld("BTCUSDT", statisticMoment(9, 55))
			underTest.statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]vo.ContractPositionStatisticVo{venueStatistic("BTCUSDT", statisticMoment(10, 0))}, nil)
			underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
				Return(0, errors.New("storage unreachable"))
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPositionStatisticServiceUnderTest(t)
			underTest.watching("BTCUSDT")
			testCase.arrange(underTest)

			report, roundError := underTest.service.RunRound(t.Context())

			require.NoError(t, roundError)
			assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "storage unreachable")
		})
	}
}

func TestPositionStatisticRoundAsksNothingOfAnEmptyWatchlist(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching()

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Empty(t, report.SymbolReports)
}

func TestPositionStatisticRoundDoesNotRunWhenTheWatchlistCannotBeRead(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, errors.New("storage unreachable"))

	_, roundError := underTest.service.RunRound(t.Context())

	assert.ErrorContains(t, roundError, "storage unreachable")
}

func TestPositionStatisticRoundForOneContract(t *testing.T) {
	testCases := []struct {
		name           string
		symbol         string
		arrange        func(underTest positionStatisticServiceUnderTest)
		expectedError  error
		expectedStored int
	}{
		{name: "登錄過的那一檔", symbol: " btcusdt ", arrange: func(underTest positionStatisticServiceUnderTest) {
			underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
				Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT"}, true, nil)
			underTest.latestHeld("BTCUSDT", statisticMoment(9, 55))
			underTest.statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
				Return([]vo.ContractPositionStatisticVo{venueStatistic("BTCUSDT", statisticMoment(10, 0))}, nil)
			underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)
		}, expectedStored: 1},
		{name: "沒有代號", symbol: "", arrange: func(positionStatisticServiceUnderTest) {},
			expectedError: domains.ErrTradingSymbolNamed},
		{name: "沒有登錄", symbol: "BTCUSDT", arrange: func(underTest positionStatisticServiceUnderTest) {
			underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
				Return(entities.ContractTradingSymbol{}, false, nil)
		}, expectedError: domains.ErrTradingSymbolNotRegistered},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPositionStatisticServiceUnderTest(t)
			testCase.arrange(underTest)

			symbolReport, catchUpError := underTest.service.RunRoundFor(t.Context(), testCase.symbol)

			if testCase.expectedError != nil {
				assert.ErrorIs(t, catchUpError, testCase.expectedError)

				return
			}
			require.NoError(t, catchUpError)
			assert.Equal(t, testCase.expectedStored, symbolReport.StoredCount)
		})
	}
}

func TestPositionStatisticRoundForOneContractPassesAStorageFailureOn(t *testing.T) {
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, errors.New("storage unreachable"))

	_, catchUpError := underTest.service.RunRoundFor(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, catchUpError, "storage unreachable")
}

func TestPositionStatisticsReadBack(t *testing.T) {
	testCases := []struct {
		name            string
		queryDto        dto.KCandleQueryDto
		held            []entities.ContractPositionStatistic
		heldError       error
		expectedCount   int
		expectedMessage string
		isValidation    bool
	}{
		{name: "由早到晚", queryDto: dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: statisticMoment(9, 0), EndTime: statisticMoment(9, 5)},
			held: []entities.ContractPositionStatistic{
				{Symbol: "BTCUSDT", StatisticTime: statisticMoment(9, 0)},
				{Symbol: "BTCUSDT", StatisticTime: statisticMoment(9, 5)},
			}, expectedCount: 2},
		{name: "沒指定合約標的", queryDto: dto.KCandleQueryDto{StartTime: statisticMoment(9, 0), EndTime: statisticMoment(9, 5)},
			expectedMessage: "必須指定交易標的", isValidation: true},
		{name: "超過上限", queryDto: dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: statisticMoment(9, 0), EndTime: statisticMoment(9, 5)},
			held: make([]entities.ContractPositionStatistic, 4), expectedMessage: "單次最多 3 筆", isValidation: true},
		{name: "讀不到", queryDto: dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: statisticMoment(9, 0), EndTime: statisticMoment(9, 5)},
			heldError: errors.New("storage unreachable"), expectedMessage: "storage unreachable"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPositionStatisticServiceUnderTest(t)
			if testCase.queryDto.Symbol != "" {
				underTest.statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 4).
					Return(testCase.held, testCase.heldError)
			}

			statistics, findError := underTest.service.FindStatisticsInRange(t.Context(), testCase.queryDto)

			if testCase.expectedMessage != "" {
				assert.ErrorContains(t, findError, testCase.expectedMessage)
				assert.Equal(t, testCase.isValidation, errors.Is(findError, domains.ErrContractPositionStatisticValidation))

				return
			}
			require.NoError(t, findError)
			require.Len(t, statistics, testCase.expectedCount)
			assert.Equal(t, statisticMoment(9, 0), statistics[0].StatisticTime)
			assert.Equal(t, statisticMoment(9, 5), statistics[1].StatisticTime)
		})
	}
}

func TestPositionStatisticRoundAsksAgainAboutAMomentSkippedBehindOnesThatWereStored(t *testing.T) {
	// Last round, 09:05 was missing a split and was skipped while 09:10 was stored.
	// This round, 09:05 has all three answers: it has to be asked about and stored.
	underTest := newPositionStatisticServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", statisticMoment(9, 10))
	underTest.statisticProxy.EXPECT().FetchPositionStatistics(
		gomock.Any(), "BTCUSDT", statisticMoment(8, 15), statisticMoment(10, 0)).
		Return([]vo.ContractPositionStatisticVo{
			venueStatistic("BTCUSDT", statisticMoment(9, 5)),
			venueStatistic("BTCUSDT", statisticMoment(9, 10)),
		}, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, statistics []entities.ContractPositionStatistic) (int, error) {
			require.Len(t, statistics, 2)
			assert.Equal(t, statisticMoment(9, 5), statistics[0].StatisticTime)

			// 09:10 is already held and left as it was; 09:05 is the one stored.
			return 1, nil
		})

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
}
