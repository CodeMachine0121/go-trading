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

var fundingRoundTime = time.Date(2026, 9, 23, 9, 30, 0, 0, time.UTC)

func settledAt(hour int) time.Time {
	return time.Date(2026, 9, 23, hour, 0, 0, 0, time.UTC)
}

func venueSettlement(symbol string, settlementTime time.Time, markPrice string) vo.ContractFundingRateSettlementVo {
	return vo.ContractFundingRateSettlementVo{
		Symbol:         symbol,
		SettlementTime: settlementTime,
		FundingRate:    decimal.RequireFromString("0.0001"),
		MarkPrice:      decimal.NewNullDecimal(decimal.RequireFromString(markPrice)),
	}
}

type fundingRateServiceUnderTest struct {
	service              *service.ContractFundingRateService
	settlementRepository *mocks.MockIContractFundingRateSettlementRepository
	symbolRepository     *mocks.MockIContractTradingSymbolRepository
	fundingRateProxy     *mocks.MockIContractFundingRateProxy
}

func newFundingRateServiceUnderTest(t *testing.T) fundingRateServiceUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	fundingRateProxy := mocks.NewMockIContractFundingRateProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(fundingRoundTime).AnyTimes()

	return fundingRateServiceUnderTest{
		service: service.NewContractFundingRateService(
			settlementRepository, symbolRepository, fundingRateProxy, clockProxy, 3),
		settlementRepository: settlementRepository,
		symbolRepository:     symbolRepository,
		fundingRateProxy:     fundingRateProxy,
	}
}

func (underTest fundingRateServiceUnderTest) watching(symbols ...string) {
	watchedSymbols := make([]entities.ContractTradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true})
	}
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchedSymbols, nil)
}

func (underTest fundingRateServiceUnderTest) latestHeld(symbol string, settlementTime time.Time) {
	underTest.settlementRepository.EXPECT().FindLatest(gomock.Any(), symbol).Return(
		entities.ContractFundingRateSettlement{Symbol: symbol, SettlementTime: settlementTime}, true, nil)
}

func (underTest fundingRateServiceUnderTest) nothingHeld(symbol string) {
	underTest.settlementRepository.EXPECT().FindLatest(gomock.Any(), symbol).Return(
		entities.ContractFundingRateSettlement{}, false, nil)
}

func TestFundingRateRoundAsksOnlyForWhatCameAfterTheLastHeldSettlement(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", settledAt(0))
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", settledAt(0)).
		Return([]vo.ContractFundingRateSettlementVo{venueSettlement("BTCUSDT", settledAt(8), "87000")}, nil)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, settlements []entities.ContractFundingRateSettlement) (int, error) {
			require.Len(t, settlements, 1)
			assert.Equal(t, settledAt(8), settlements[0].SettlementTime)

			return 1, nil
		})

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, "BTCUSDT", report.SymbolReports[0].Symbol)
	assert.Equal(t, 1, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
}

func TestFundingRateRoundAsksFromTheFirstSettlementWhenNothingIsHeld(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.nothingHeld("BTCUSDT")
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", time.Time{}).
		Return(nil, nil)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(0)).Return(0, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Empty(t, report.SymbolReports[0].FetchFailureReason)
	assert.Equal(t, 0, report.SymbolReports[0].SkippedCount)
}

func TestFundingRateRoundSkipsTheSettlementThatBreaksARuleAndStoresTheRest(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.watching("BTCUSDT")
	underTest.latestHeld("BTCUSDT", settledAt(0))
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]vo.ContractFundingRateSettlementVo{
			venueSettlement("BTCUSDT", settledAt(4), "0"),
			venueSettlement("BTCUSDT", settledAt(8), "87000"),
		}, nil)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	symbolReport := report.SymbolReports[0]
	assert.Equal(t, 1, symbolReport.StoredCount)
	assert.Equal(t, 1, symbolReport.SkippedCount)
	require.Len(t, symbolReport.SkippedRecords, 1)
	assert.Equal(t, settledAt(4), symbolReport.SkippedRecords[0].RecordTime)
	assert.Contains(t, symbolReport.SkippedRecords[0].Reason, "結算當下的標記價格必須大於零")
}

func TestFundingRateRoundLeavesOneContractsFailureToItself(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.watching("BTCUSDT", "ETHUSDT")
	underTest.latestHeld("BTCUSDT", settledAt(0))
	underTest.latestHeld("ETHUSDT", settledAt(0))
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", gomock.Any()).
		Return(nil, errors.New("venue unreachable"))
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "ETHUSDT", gomock.Any()).
		Return([]vo.ContractFundingRateSettlementVo{venueSettlement("ETHUSDT", settledAt(8), "3000")}, nil)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	require.Len(t, report.SymbolReports, 2)
	assert.Equal(t, "BTCUSDT", report.SymbolReports[0].Symbol)
	assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "venue unreachable")
	assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
	assert.Equal(t, 1, report.SymbolReports[1].StoredCount)
	assert.Empty(t, report.SymbolReports[1].FetchFailureReason)
}

func TestFundingRateRoundReportsStorageFailingForThatContract(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest fundingRateServiceUnderTest)
	}{
		{name: "讀不到上一次存到哪", arrange: func(underTest fundingRateServiceUnderTest) {
			underTest.settlementRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
				Return(entities.ContractFundingRateSettlement{}, false, errors.New("storage unreachable"))
		}},
		{name: "存不進去", arrange: func(underTest fundingRateServiceUnderTest) {
			underTest.latestHeld("BTCUSDT", settledAt(0))
			underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]vo.ContractFundingRateSettlementVo{venueSettlement("BTCUSDT", settledAt(8), "1")}, nil)
			underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
				Return(0, errors.New("storage unreachable"))
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newFundingRateServiceUnderTest(t)
			underTest.watching("BTCUSDT")
			testCase.arrange(underTest)

			report, roundError := underTest.service.RunRound(t.Context())

			require.NoError(t, roundError)
			assert.Contains(t, report.SymbolReports[0].FetchFailureReason, "storage unreachable")
			assert.Equal(t, 0, report.SymbolReports[0].StoredCount)
		})
	}
}

func TestFundingRateRoundAsksNothingOfAnEmptyWatchlist(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.watching()

	report, roundError := underTest.service.RunRound(t.Context())

	require.NoError(t, roundError)
	assert.Empty(t, report.SymbolReports)
}

func TestFundingRateRoundDoesNotRunWhenTheWatchlistCannotBeRead(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.symbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, errors.New("storage unreachable"))

	_, roundError := underTest.service.RunRound(t.Context())

	assert.ErrorContains(t, roundError, "storage unreachable")
}

func TestFundingRateRoundForOneContractCatchesUpARegisteredOne(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	underTest.nothingHeld("BTCUSDT")
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "BTCUSDT", time.Time{}).
		Return([]vo.ContractFundingRateSettlementVo{venueSettlement("BTCUSDT", settledAt(8), "1")}, nil)
	underTest.settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	symbolReport, catchUpError := underTest.service.RunRoundFor(t.Context(), " btcusdt ")

	require.NoError(t, catchUpError)
	assert.Equal(t, "BTCUSDT", symbolReport.Symbol)
	assert.Equal(t, 1, symbolReport.StoredCount)
}

func TestFundingRateRoundForOneContractRefusesWhatItCannotReach(t *testing.T) {
	testCases := []struct {
		name          string
		symbol        string
		arrange       func(underTest fundingRateServiceUnderTest)
		expectedError error
	}{
		{name: "沒有代號", symbol: "  ", arrange: func(fundingRateServiceUnderTest) {},
			expectedError: domains.ErrTradingSymbolNamed},
		{name: "沒有登錄", symbol: "BTCUSDT", arrange: func(underTest fundingRateServiceUnderTest) {
			underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
				Return(entities.ContractTradingSymbol{}, false, nil)
		}, expectedError: domains.ErrTradingSymbolNotRegistered},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newFundingRateServiceUnderTest(t)
			testCase.arrange(underTest)

			_, catchUpError := underTest.service.RunRoundFor(t.Context(), testCase.symbol)

			assert.ErrorIs(t, catchUpError, testCase.expectedError)
		})
	}
}

func TestFundingRateRoundForOneContractPassesAStorageFailureOn(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, errors.New("storage unreachable"))

	_, catchUpError := underTest.service.RunRoundFor(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, catchUpError, "storage unreachable")
}

func TestFundingRateSettlementsReadBackEarliestFirst(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 4).Return(
		[]entities.ContractFundingRateSettlement{
			{Symbol: "BTCUSDT", SettlementTime: settledAt(0), FundingRate: decimal.RequireFromString("0.0001")},
			{Symbol: "BTCUSDT", SettlementTime: settledAt(8), FundingRate: decimal.RequireFromString("-0.0002")},
		}, nil)

	settlements, findError := underTest.service.FindSettlementsInRange(t.Context(), dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: settledAt(0), EndTime: settledAt(8),
	})

	require.NoError(t, findError)
	require.Len(t, settlements, 2)
	assert.Equal(t, settledAt(0), settlements[0].SettlementTime)
	assert.True(t, decimal.RequireFromString("-0.0002").Equal(settlements[1].FundingRate))
}

func TestFundingRateSettlementsRefuseAQueryTheyCannotAnswer(t *testing.T) {
	testCases := []struct {
		name            string
		queryDto        dto.KCandleQueryDto
		arrange         func(underTest fundingRateServiceUnderTest)
		expectedMessage string
	}{
		{name: "沒指定合約標的", queryDto: dto.KCandleQueryDto{StartTime: settledAt(0), EndTime: settledAt(8)},
			arrange: func(fundingRateServiceUnderTest) {}, expectedMessage: "必須指定交易標的"},
		{name: "區間裡超過上限", queryDto: dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: settledAt(0), EndTime: settledAt(8)},
			arrange: func(underTest fundingRateServiceUnderTest) {
				underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(make([]entities.ContractFundingRateSettlement, 4), nil)
			}, expectedMessage: "單次最多 3 筆"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newFundingRateServiceUnderTest(t)
			testCase.arrange(underTest)

			_, findError := underTest.service.FindSettlementsInRange(t.Context(), testCase.queryDto)

			assert.ErrorIs(t, findError, domains.ErrContractFundingRateSettlementValidation)
			assert.ErrorContains(t, findError, testCase.expectedMessage)
		})
	}
}

func TestFundingRateSettlementsPassAStorageFailureOn(t *testing.T) {
	underTest := newFundingRateServiceUnderTest(t)
	underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage unreachable"))

	_, findError := underTest.service.FindSettlementsInRange(t.Context(), dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: settledAt(0), EndTime: settledAt(8),
	})

	assert.ErrorContains(t, findError, "storage unreachable")
	assert.NotErrorIs(t, findError, domains.ErrContractFundingRateSettlementValidation)
}
