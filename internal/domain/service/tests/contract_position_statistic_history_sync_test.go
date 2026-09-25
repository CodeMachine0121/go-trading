package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	statisticSyncAt   = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	statisticDayOne   = time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	statisticDayTwo   = time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	archiveUnreadable = errors.New("position statistic archive answered 500")
	statisticsBroken  = errors.New("statistics storage went away")
)

func archivedAt(statisticTime time.Time) vo.ContractPositionStatisticArchiveVo {
	return vo.ContractPositionStatisticArchiveVo{
		Symbol:                          "BTCUSDT",
		StatisticTime:                   statisticTime,
		OpenInterest:                    decimal.NewNullDecimal(decimal.RequireFromString("106479.865")),
		OpenInterestValue:               decimal.NewNullDecimal(decimal.RequireFromString("9262820059.48")),
		AccountLongShortRatio:           decimal.NewNullDecimal(decimal.RequireFromString("3")),
		TopTraderPositionLongShortRatio: decimal.NewNullDecimal(decimal.RequireFromString("1")),
	}
}

func archivedWholeDay(day time.Time) []vo.ContractPositionStatisticArchiveVo {
	statistics := make([]vo.ContractPositionStatisticArchiveVo, 0, 288)
	for index := range 288 {
		statistics = append(statistics, archivedAt(day.Add(time.Duration(index)*5*time.Minute)))
	}

	return statistics
}

// contractSyncRunWrites keeps every write the background walk makes to its run, in
// order, so a case can look at what the run said while it was still going.
type contractSyncRunWrites struct {
	lock   *sync.Mutex
	writes *[]entities.KCandleContractHistorySyncRun
	ended  chan entities.KCandleContractHistorySyncRun
}

func (underTest contractIngestionUnderTest) keepsEveryContractSyncRunWrite() contractSyncRunWrites {
	runWrites := contractSyncRunWrites{
		lock:   &sync.Mutex{},
		writes: &[]entities.KCandleContractHistorySyncRun{},
		ended:  make(chan entities.KCandleContractHistorySyncRun, 1),
	}
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, syncRun entities.KCandleContractHistorySyncRun,
		) (entities.KCandleContractHistorySyncRun, error) {
			if syncRun.ID == 0 {
				syncRun.ID = 1
			}
			runWrites.lock.Lock()
			*runWrites.writes = append(*runWrites.writes, syncRun)
			runWrites.lock.Unlock()
			if syncRun.FinishedAt != nil {
				runWrites.ended <- syncRun
			}

			return syncRun, nil
		}).AnyTimes()

	return runWrites
}

func (runWrites contractSyncRunWrites) latest() entities.KCandleContractHistorySyncRun {
	runWrites.lock.Lock()
	defer runWrites.lock.Unlock()

	return (*runWrites.writes)[len(*runWrites.writes)-1]
}

func (runWrites contractSyncRunWrites) first() entities.KCandleContractHistorySyncRun {
	runWrites.lock.Lock()
	defer runWrites.lock.Unlock()

	return (*runWrites.writes)[0]
}

func (runWrites contractSyncRunWrites) awaitEnding(t *testing.T) entities.KCandleContractHistorySyncRun {
	t.Helper()

	select {
	case endedRun := <-runWrites.ended:
		return endedRun
	case <-time.After(5 * time.Second):
		t.Fatal("合約歷史同步沒有收尾")

		return entities.KCandleContractHistorySyncRun{}
	}
}

// candlesAlreadyWhole makes the candle half of the run a walk that asks nothing, so a
// case is only about the statistics.
func (underTest contractIngestionUnderTest) candlesAlreadyWhole() {
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(1000000, nil).AnyTimes()
}

func (underTest contractIngestionUnderTest) holdingStatistics(day time.Time, heldCount int) {
	underTest.statisticRepository.EXPECT().
		CountInRange(gomock.Any(), "BTCUSDT", day, day.Add(23*time.Hour+55*time.Minute)).
		Return(heldCount, nil).AnyTimes()
}

func (underTest contractIngestionUnderTest) archiveAnswers(
	day time.Time, statistics []vo.ContractPositionStatisticArchiveVo, found bool, fetchError error,
) *gomock.Call {
	return underTest.archiveProxy.EXPECT().
		FetchDailyPositionStatistics(gomock.Any(), "BTCUSDT", day).Return(statistics, found, fetchError)
}

func (underTest contractIngestionUnderTest) startSyncing(t *testing.T, lookbackDays int) dto.KCandleContractHistorySyncRunDto {
	t.Helper()

	startedRun, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: lookbackDays},
		contractHistoryCeilingDays)
	require.NoError(t, startError)

	return startedRun
}

func TestContractHistorySyncSaysHowManyDaysOfStatisticsItWillWalk(t *testing.T) {
	testCases := []struct {
		name             string
		lookbackDays     int
		expectedDayCount int
	}{
		{name: "回溯 180 天", lookbackDays: 180, expectedDayCount: 181},
		{name: "回溯 1 天", lookbackDays: 1, expectedDayCount: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newContractIngestionUnderTest(t, statisticSyncAt)
			underTest.registered("BTCUSDT")
			underTest.candlesAlreadyWhole()
			runWrites := underTest.keepsEveryContractSyncRunWrite()

			startedRun := underTest.startSyncing(t, testCase.lookbackDays)

			assert.Equal(t, testCase.expectedDayCount, startedRun.PositionStatistic.TotalDays)
			assert.Equal(t, testCase.expectedDayCount, runWrites.first().PositionStatisticTotalDays)
			endedRun := runWrites.awaitEnding(t)
			assert.Equal(t, testCase.expectedDayCount, endedRun.PositionStatisticTotalDays)
		})
	}
}

func TestContractHistorySyncFillsADayOfStatisticsInFromTheArchive(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 0)
	underTest.archiveAnswers(statisticDayOne, archivedWholeDay(statisticDayOne), true, nil)
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil)
	savedStatistics := make([]entities.ContractPositionStatistic, 0)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, statistics []entities.ContractPositionStatistic) (int, error) {
			savedStatistics = append(savedStatistics, statistics...)

			return len(statistics), nil
		})

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	require.Len(t, savedStatistics, 288)
	assert.Equal(t, statisticDayOne, savedStatistics[0].StatisticTime)
	// A ratio of 3 is three parts long in every four, and 1 is half and half.
	assert.True(t, decimal.RequireFromString("0.75").Equal(savedStatistics[0].AccountLongShare))
	assert.True(t, decimal.RequireFromString("0.25").Equal(savedStatistics[0].AccountShortShare))
	assert.True(t, decimal.RequireFromString("0.5").Equal(savedStatistics[0].TopTraderPositionLongShare))
	assert.Equal(t, 288, endedRun.PositionStatisticStoredCount)
	assert.Equal(t, 0, endedRun.PositionStatisticSkippedCount)
	assert.Equal(t, 2, endedRun.PositionStatisticCompletedDays)
	assert.Empty(t, endedRun.PositionStatisticFetchFailureReason)
}

func TestContractHistorySyncDoesNotAskTheArchiveAboutADayItHoldsWhole(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.holdingStatistics(statisticDayOne, 288)
	underTest.holdingStatistics(statisticDayTwo, 0)
	underTest.archiveAnswers(statisticDayOne, nil, false, nil).Times(0)
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil).Times(1)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, 2, endedRun.PositionStatisticCompletedDays)
	assert.Equal(t, 0, endedRun.PositionStatisticStoredCount)
}

func TestContractHistorySyncStoresOnlyTheStatisticsADayIsMissing(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.holdingStatistics(statisticDayOne, 287)
	underTest.holdingStatistics(statisticDayTwo, 288)
	underTest.archiveAnswers(statisticDayOne, archivedWholeDay(statisticDayOne), true, nil).Times(1)
	// Storage keeps what it already holds and says how many it added: the one missing.
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(288)).Return(1, nil)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, 1, endedRun.PositionStatisticStoredCount)
}

func TestContractHistorySyncSkipsTheArchivedStatisticsThatBreakARule(t *testing.T) {
	negativeRatio := archivedAt(statisticDayOne.Add(5 * time.Minute))
	negativeRatio.AccountLongShortRatio = decimal.NewNullDecimal(decimal.RequireFromString("-1"))
	missingTopTrader := archivedAt(statisticDayOne.Add(10 * time.Minute))
	missingTopTrader.TopTraderPositionLongShortRatio = decimal.NullDecimal{}
	negativeOpenInterest := archivedAt(statisticDayOne.Add(15 * time.Minute))
	negativeOpenInterest.OpenInterest = decimal.NewNullDecimal(decimal.RequireFromString("-1"))
	offTheGrid := archivedAt(statisticDayOne.Add(18 * time.Minute))

	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 288)
	underTest.archiveAnswers(statisticDayOne, []vo.ContractPositionStatisticArchiveVo{
		archivedAt(statisticDayOne), negativeRatio, missingTopTrader, negativeOpenInterest, offTheGrid,
	}, true, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Len(1)).Return(1, nil)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Equal(t, 1, endedRun.PositionStatisticStoredCount)
	assert.Equal(t, 4, endedRun.PositionStatisticSkippedCount)
}

func TestContractHistorySyncPassesTheDaysTheArchiveHasNoFileFor(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 0)
	underTest.archiveAnswers(statisticDayOne, nil, false, nil).Times(1)
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil).Times(1)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Equal(t, 0, endedRun.PositionStatisticStoredCount)
	assert.Equal(t, 2, endedRun.PositionStatisticCompletedDays)
	assert.Empty(t, endedRun.PositionStatisticFetchFailureReason)
	assert.Empty(t, endedRun.FailureReason)
}

func TestContractHistorySyncStopsTheStatisticsWhenTheArchiveRefusesWithoutFailingTheRun(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	underTest.candlesAlreadyWhole()
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	days := make([]time.Time, 0, 5)
	for dayIndex := range 5 {
		day := time.Date(2026, 9, 21+dayIndex, 0, 0, 0, 0, time.UTC)
		days = append(days, day)
		underTest.holdingStatistics(day, 0)
	}
	underTest.archiveAnswers(days[0], nil, false, nil)
	underTest.archiveAnswers(days[1], nil, false, nil)
	underTest.archiveAnswers(days[2], nil, false, archiveUnreadable)
	underTest.archiveAnswers(days[3], nil, false, nil).Times(0)
	underTest.archiveAnswers(days[4], nil, false, nil).Times(0)

	underTest.startSyncing(t, 4)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Empty(t, endedRun.FailureReason)
	assert.Equal(t, 5, endedRun.PositionStatisticTotalDays)
	assert.Equal(t, 2, endedRun.PositionStatisticCompletedDays)
	assert.Contains(t, endedRun.PositionStatisticFetchFailureReason, archiveUnreadable.Error())
	assert.Empty(t, endedRun.FetchFailureReason)
}

func TestContractHistorySyncStillFillsTheStatisticsInWhenTheCandleSourceRefuses(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable).AnyTimes()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 0)
	underTest.archiveAnswers(statisticDayOne, nil, false, nil).Times(1)
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil).Times(1)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Contains(t, endedRun.FetchFailureReason, sourceUnreachable.Error())
	assert.Equal(t, 2, endedRun.PositionStatisticCompletedDays)
	assert.Empty(t, endedRun.PositionStatisticFetchFailureReason)
}

func TestContractHistorySyncKeepsTheTwoSourcesRefusalsApart(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable).AnyTimes()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.archiveAnswers(statisticDayOne, nil, false, archiveUnreadable)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), endedRun.Status)
	assert.Contains(t, endedRun.FetchFailureReason, sourceUnreachable.Error())
	assert.NotContains(t, endedRun.FetchFailureReason, archiveUnreadable.Error())
	assert.Contains(t, endedRun.PositionStatisticFetchFailureReason, archiveUnreadable.Error())
	assert.NotContains(t, endedRun.PositionStatisticFetchFailureReason, sourceUnreachable.Error())
}

func TestContractHistorySyncFailsTheRunWhenStatisticStorageBreaks(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest contractIngestionUnderTest)
	}{
		{name: "存不進去", arrange: func(underTest contractIngestionUnderTest) {
			underTest.holdingStatistics(statisticDayOne, 0)
			underTest.archiveAnswers(statisticDayOne, archivedWholeDay(statisticDayOne), true, nil)
			underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).
				Return(0, statisticsBroken)
		}},
		{name: "讀不到自己存了幾筆", arrange: func(underTest contractIngestionUnderTest) {
			underTest.statisticRepository.EXPECT().
				CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, statisticsBroken)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
			underTest.registered("BTCUSDT")
			underTest.candlesAlreadyWhole()
			runWrites := underTest.keepsEveryContractSyncRunWrite()
			testCase.arrange(underTest)

			underTest.startSyncing(t, 1)

			endedRun := runWrites.awaitEnding(t)
			assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
			assert.Contains(t, endedRun.FailureReason, statisticsBroken.Error())
		})
	}
}

func TestContractHistorySyncDoesNotReachTheStatisticsWhenTheCandlesBrokeTheSystem(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.kCandleContractRepository.EXPECT().
		CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(0, statisticsBroken)
	underTest.archiveProxy.EXPECT().
		FetchDailyPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	underTest.startSyncing(t, 1)

	endedRun := runWrites.awaitEnding(t)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), endedRun.Status)
	assert.Equal(t, 0, endedRun.PositionStatisticCompletedDays)
}

func TestContractHistorySyncWalksTheStatisticsOnlyOnceTheCandlesAreDone(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.candlesAlreadyWhole()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 0)
	candleChunksDoneAtFirstArchiveQuestion := make([]bool, 0, 2)
	askedArchive := func(_ context.Context, _ string, _ time.Time) ([]vo.ContractPositionStatisticArchiveVo, bool, error) {
		latest := runWrites.latest()
		candleChunksDoneAtFirstArchiveQuestion = append(candleChunksDoneAtFirstArchiveQuestion,
			latest.CompletedChunks == latest.TotalChunks)

		return nil, false, nil
	}
	underTest.archiveAnswers(statisticDayOne, nil, false, nil).DoAndReturn(askedArchive)
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil).DoAndReturn(askedArchive)

	underTest.startSyncing(t, 1)

	runWrites.awaitEnding(t)
	assert.Equal(t, []bool{true, true}, candleChunksDoneAtFirstArchiveQuestion)
}

func TestContractHistorySyncWritesHowManyDaysAreBehindItBeforeAskingAboutTheNext(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.registered("BTCUSDT")
	runWrites := underTest.keepsEveryContractSyncRunWrite()
	underTest.candlesAlreadyWhole()
	underTest.holdingStatistics(statisticDayOne, 0)
	underTest.holdingStatistics(statisticDayTwo, 0)
	underTest.archiveAnswers(statisticDayOne, archivedWholeDay(statisticDayOne), true, nil)
	underTest.statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(288, nil)
	progressWhenAskingAboutDayTwo := dto.ContractPositionStatisticSyncProgressDto{}
	underTest.archiveAnswers(statisticDayTwo, nil, false, nil).DoAndReturn(
		func(_ context.Context, _ string, _ time.Time) ([]vo.ContractPositionStatisticArchiveVo, bool, error) {
			latest := runWrites.latest()
			progressWhenAskingAboutDayTwo = dto.ContractPositionStatisticSyncProgressDto{
				CompletedDays: latest.PositionStatisticCompletedDays,
				StoredCount:   latest.PositionStatisticStoredCount,
			}

			return nil, false, nil
		})

	underTest.startSyncing(t, 1)

	runWrites.awaitEnding(t)
	assert.Equal(t, 1, progressWhenAskingAboutDayTwo.CompletedDays)
	assert.Equal(t, 288, progressWhenAskingAboutDayTwo.StoredCount)
}

func TestContractHistorySyncRefusingALookbackAsksTheArchiveNothing(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	underTest.archiveProxy.EXPECT().
		FetchDailyPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, startError := underTest.service.StartHistorySyncFor(
		t.Context(), dto.KCandleHistorySyncDto{Symbol: "BTCUSDT", LookbackDays: contractHistoryCeilingDays + 1},
		contractHistoryCeilingDays)

	require.Error(t, startError)
	assert.Contains(t, startError.Error(), "3650")
}

func TestContractHistorySyncRunReadsBackWithBothGroupsApart(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
		entities.KCandleContractHistorySyncRun{
			ID: 7, Symbol: "BTCUSDT", Status: string(vo.KCandleHistorySyncRunning),
			StoredCount: 1000, SkippedCount: 2, FetchFailureReason: "candles refused",
			PositionStatisticTotalDays: 181, PositionStatisticCompletedDays: 20,
			PositionStatisticStoredCount: 500, PositionStatisticSkippedCount: 3,
			PositionStatisticFetchFailureReason: "archive refused",
		}, true, nil)

	readRun, readError := underTest.service.GetHistorySyncRun(t.Context(), 7)

	require.NoError(t, readError)
	assert.Equal(t, 1000, readRun.StoredCount)
	assert.Equal(t, 2, readRun.SkippedCount)
	assert.Equal(t, "candles refused", readRun.FetchFailureReason)
	assert.Equal(t, dto.ContractPositionStatisticSyncProgressDto{
		TotalDays: 181, CompletedDays: 20, StoredCount: 500, SkippedCount: 3,
		FetchFailureReason: "archive refused",
	}, readRun.PositionStatistic)
}

func TestContractHistorySyncRunCannotBeReadWhenStorageBreaks(t *testing.T) {
	underTest := newContractHistorySyncUnderTest(t, statisticSyncAt)
	underTest.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
		Return(entities.KCandleContractHistorySyncRun{}, false, statisticsBroken)

	_, readError := underTest.service.GetHistorySyncRun(t.Context(), 7)

	assert.ErrorIs(t, readError, statisticsBroken)
}
