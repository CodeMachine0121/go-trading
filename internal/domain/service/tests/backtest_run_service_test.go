package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
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

const claimedBacktestRunID = uint(23)

var backtestRunServiceNow = contractBacktestServiceStart.Add(24 * time.Hour)

// scriptReplay stands in for the script proxy: given the replay's context and progress sink, it answers per-bar values.
type scriptReplay func(
	executionContext context.Context, kCandles []vo.KCandleVo, progress domaininterface.IReplayProgressSink,
) ([]map[string]vo.IndicatorValueVo, error)

type backtestRunServiceUnderTest struct {
	backtestRunService *service.BacktestRunService
	runRepository      *mocks.MockIBacktestRunRepository
	resultRepository   *mocks.MockIBacktestRunResultRepository
	writesMutex        sync.Mutex
	writes             []entities.BacktestRun
	// stillRunning answers each conditional write; false is a run cancelled or swept meanwhile.
	stillRunning func(write entities.BacktestRun) bool
}

func newBacktestRunServiceUnderTest(
	t *testing.T, timeAllowance time.Duration, progressWriteEveryBars int, replay scriptReplay,
) *backtestRunServiceUnderTest {
	t.Helper()
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestRunServiceNow).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, domains.KCandleQueryDomain, int) ([]entities.KCandle, error) {
			kCandles := make([]entities.KCandle, 0, 3)
			for hour, closePrice := range []int64{100, 110, 120} {
				price := decimal.NewFromInt(closePrice)
				kCandles = append(kCandles, entities.KCandle{
					Symbol: "BTCUSDT", OpenTime: contractBacktestServiceStart.Add(time.Duration(hour) * time.Hour),
					Open: price, High: price, Low: price, Close: price,
				})
			}
			return kCandles, nil
		}).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			kCandles []vo.KCandleVo, _ domains.StrategyScriptParametersDomain, progress domaininterface.IReplayProgressSink,
		) ([]map[string]vo.IndicatorValueVo, error) {
			return replay(executionContext, kCandles, progress)
		}).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()

	fixture := &backtestRunServiceUnderTest{
		runRepository:    mocks.NewMockIBacktestRunRepository(controller),
		resultRepository: mocks.NewMockIBacktestRunResultRepository(controller),
		stillRunning:     func(entities.BacktestRun) bool { return true },
	}
	fixture.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, write entities.BacktestRun) (bool, error) {
			fixture.writesMutex.Lock()
			defer fixture.writesMutex.Unlock()
			fixture.writes = append(fixture.writes, write)
			return fixture.stillRunning(write), nil
		}).AnyTimes()
	fixture.backtestRunService = service.NewBacktestRunService(
		fixture.runRepository, fixture.resultRepository, transactionRepository, clockProxy,
		service.NewBacktestService(kCandleRepository, indicatorScriptProxy, clockProxy, 1000, 0),
		newContractBacktestServiceUnderTest(t, func(contractBacktestServiceMocks) {}),
		domains.NewBacktestRunAdmissionDomain(3), "replica-a", timeAllowance, progressWriteEveryBars)

	return fixture
}

// queueHolds makes the next claim hand over a run of the given kind and request.
func (fixture *backtestRunServiceUnderTest) queueHolds(kind vo.BacktestRunKindVo, request dto.BacktestRunRequestDto) {
	startedAt := backtestRunServiceNow
	fixture.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), "replica-a", backtestRunServiceNow).
		Return(entities.BacktestRun{
			ID: claimedBacktestRunID, OwnerID: 1, Kind: string(kind), Request: request,
			Status: string(vo.BacktestRunRunning), ReplicaName: "replica-a", StartedAt: &startedAt,
		}, true, nil)
}

func (fixture *backtestRunServiceUnderTest) claimAndWaitForTheEnding(t *testing.T) entities.BacktestRun {
	t.Helper()
	claimed, err := fixture.backtestRunService.ClaimAndRun(t.Context())
	require.NoError(t, err)
	require.True(t, claimed)
	require.Eventually(t, func() bool { return fixture.backtestRunService.RunningCount() == 0 },
		5*time.Second, 5*time.Millisecond)

	return fixture.lastWrite(t)
}

func (fixture *backtestRunServiceUnderTest) lastWrite(t *testing.T) entities.BacktestRun {
	t.Helper()
	fixture.writesMutex.Lock()
	defer fixture.writesMutex.Unlock()
	require.NotEmpty(t, fixture.writes)

	return fixture.writes[len(fixture.writes)-1]
}

func spotScriptRun() dto.BacktestRunRequestDto {
	return dto.BacktestRunRequestDto{SpotScript: &dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: contractBacktestServiceStart, EndTime: contractBacktestServiceStart.Add(4 * time.Hour),
		Script: "the script", InitialCapital: decimal.NewFromInt(10000), PositionSizingMode: "allIn",
	}}
}

// buyingThenSelling reports every bar to the sink, buying on the first and selling on the second.
func buyingThenSelling(
	_ context.Context, kCandles []vo.KCandleVo, progress domaininterface.IReplayProgressSink,
) ([]map[string]vo.IndicatorValueVo, error) {
	perBar := make([]map[string]vo.IndicatorValueVo, 0, len(kCandles))
	for barIndex := range kCandles {
		signal := vo.SignalHold
		switch barIndex {
		case 0:
			signal = vo.SignalBuy
		case 1:
			signal = vo.SignalSell
		}
		perBar = append(perBar, map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: signal}})
		if progress != nil {
			progress.ReplayAdvanced(barIndex+1, len(kCandles))
		}
	}

	return perBar, nil
}

// waitingForTheEnd blocks like a long replay until its context ends, saying it has started first.
func waitingForTheEnd(started chan<- struct{}) scriptReplay {
	return func(executionContext context.Context, _ []vo.KCandleVo, progress domaininterface.IReplayProgressSink,
	) ([]map[string]vo.IndicatorValueVo, error) {
		progress.ReplayAdvanced(1, 3)
		close(started)
		<-executionContext.Done()

		return nil, errors.New("算式已中止，因為發動它的請求已經結束")
	}
}

func TestBacktestRunnerStoresTheReportAndTheHeadlineFiguresOfASuccess(t *testing.T) {
	fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, buyingThenSelling)
	fixture.queueHolds(vo.BacktestRunSpotScript, spotScriptRun())
	stored := entities.BacktestRunResult{}
	fixture.resultRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, result entities.BacktestRunResult) error {
			stored = result
			return nil
		})

	ending := fixture.claimAndWaitForTheEnding(t)

	assert.Equal(t, "succeeded", ending.Status)
	require.NotNil(t, ending.TotalReturnRate)
	assert.InDelta(t, 0.1, *ending.TotalReturnRate, 1e-9)
	assert.Equal(t, 1, *ending.TradeCount)
	assert.Equal(t, 3, ending.CompletedBarCount)
	assert.Equal(t, 3, ending.TotalBarCount)
	require.NotNil(t, ending.FinishedAt)
	assert.Equal(t, claimedBacktestRunID, stored.BacktestRunID)
	require.NotNil(t, stored.SpotResult)
	assert.Nil(t, stored.ContractResult)
	assert.Len(t, stored.SpotResult.ClosedTrades, 1)
}

func TestBacktestRunnerReplaysEveryKindItCanBeGiven(t *testing.T) {
	spotRequest := *spotScriptRun().SpotScript
	contractRequest := contractBacktestServiceRequest()
	contractStrategyRequest := contractTradingStrategyBacktestServiceRequest()
	testCases := []struct {
		name             string
		kind             vo.BacktestRunKindVo
		request          dto.BacktestRunRequestDto
		expectedContract bool
	}{
		{name: "a spot trading strategy", kind: vo.BacktestRunSpotTradingStrategy,
			request: dto.BacktestRunRequestDto{SpotTradingStrategy: &dto.TradingStrategyBacktestRequestDto{
				Symbol: "BTCUSDT", StartTime: spotRequest.StartTime, EndTime: spotRequest.EndTime,
				SignalSources:  []dto.ResolvedSignalSourceDto{{Label: "A", AggregationInterval: "1h", Script: "s"}},
				BuyCondition:   dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "buy"},
				SellCondition:  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "sell"},
				InitialCapital: decimal.NewFromInt(10000), PositionSizingMode: "allIn",
			}}},
		{name: "a contract script", kind: vo.BacktestRunContractScript,
			request: dto.BacktestRunRequestDto{ContractScript: &contractRequest}, expectedContract: true},
		{name: "a contract trading strategy", kind: vo.BacktestRunContractTradingStrategy,
			request: dto.BacktestRunRequestDto{ContractTradingStrategy: &contractStrategyRequest}, expectedContract: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, buyingThenSelling)
			fixture.queueHolds(testCase.kind, testCase.request)
			stored := entities.BacktestRunResult{}
			fixture.resultRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, result entities.BacktestRunResult) error {
					stored = result
					return nil
				})

			ending := fixture.claimAndWaitForTheEnding(t)

			assert.Equal(t, "succeeded", ending.Status, ending.FailureReason)
			assert.Equal(t, testCase.expectedContract, stored.ContractResult != nil)
			assert.Equal(t, !testCase.expectedContract, stored.SpotResult != nil)
		})
	}
}

func TestBacktestRunnerEndsAFailureWithItsReason(t *testing.T) {
	testCases := []struct {
		name           string
		timeAllowance  time.Duration
		kind           vo.BacktestRunKindVo
		replay         scriptReplay
		expectedReason string
	}{
		{
			name: "the script failing midway keeps its words", timeAllowance: time.Minute, kind: vo.BacktestRunSpotScript,
			replay: func(context.Context, []vo.KCandleVo, domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				return nil, errors.New("算式執行失敗：除以零")
			},
			expectedReason: "算式執行失敗：除以零",
		},
		{
			name: "running past the time allowance", timeAllowance: 20 * time.Millisecond, kind: vo.BacktestRunSpotScript,
			replay: func(executionContext context.Context, _ []vo.KCandleVo, _ domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				<-executionContext.Done()
				return nil, errors.New("stopped")
			},
			expectedReason: "超過時間上限",
		},
		{
			name: "the replay breaking down", timeAllowance: time.Minute, kind: vo.BacktestRunSpotScript,
			replay: func(context.Context, []vo.KCandleVo, domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				panic("index out of range")
			},
			expectedReason: "回測服務壞掉了，這筆沒有跑完",
		},
		{
			name: "a kind nobody knows", timeAllowance: time.Minute, kind: "optionScript", replay: buyingThenSelling,
			expectedReason: "認不得的回測種類，這筆沒有跑",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunServiceUnderTest(t, testCase.timeAllowance, 100, testCase.replay)
			fixture.queueHolds(testCase.kind, spotScriptRun())

			ending := fixture.claimAndWaitForTheEnding(t)

			assert.Equal(t, "failed", ending.Status)
			assert.Equal(t, testCase.expectedReason, ending.FailureReason)
			assert.Nil(t, ending.TotalReturnRate)
			require.NotNil(t, ending.FinishedAt)
		})
	}
}

func TestBacktestRunnerWritesProgressEverySoManyBarsAndAtTheEnd(t *testing.T) {
	fiveBars := func(_ context.Context, _ []vo.KCandleVo, progress domaininterface.IReplayProgressSink,
	) ([]map[string]vo.IndicatorValueVo, error) {
		for completed := 1; completed <= 5; completed++ {
			progress.ReplayAdvanced(completed, 5)
		}
		return nil, errors.New("stop here")
	}
	fixture := newBacktestRunServiceUnderTest(t, time.Minute, 2, fiveBars)
	fixture.queueHolds(vo.BacktestRunSpotScript, spotScriptRun())

	fixture.claimAndWaitForTheEnding(t)

	fixture.writesMutex.Lock()
	defer fixture.writesMutex.Unlock()
	progressWrites := []int{}
	for _, write := range fixture.writes[:len(fixture.writes)-1] {
		assert.Equal(t, "running", write.Status)
		assert.Equal(t, "replica-a", write.ReplicaName)
		progressWrites = append(progressWrites, write.CompletedBarCount)
	}
	assert.Equal(t, []int{2, 4, 5}, progressWrites)
}

func TestBacktestRunnerStopsAndStoresNothingOnceItsRunIsCancelled(t *testing.T) {
	started := make(chan struct{})
	fixture := newBacktestRunServiceUnderTest(t, time.Minute, 1, waitingForTheEnd(started))
	fixture.stillRunning = func(entities.BacktestRun) bool { return false }
	fixture.queueHolds(vo.BacktestRunSpotScript, spotScriptRun())

	claimed, err := fixture.backtestRunService.ClaimAndRun(t.Context())
	require.NoError(t, err)
	require.True(t, claimed)

	// No result save is expected, so storing a report would fail the test.
	require.Eventually(t, func() bool { return fixture.backtestRunService.RunningCount() == 0 },
		5*time.Second, 5*time.Millisecond)
}

func TestBacktestRunnerOnAGracefulStopMarksItsRunInterrupted(t *testing.T) {
	started := make(chan struct{})
	fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, waitingForTheEnd(started))
	fixture.queueHolds(vo.BacktestRunSpotScript, spotScriptRun())
	claimed, err := fixture.backtestRunService.ClaimAndRun(t.Context())
	require.NoError(t, err)
	require.True(t, claimed)
	<-started

	fixture.backtestRunService.StopRunners()

	assert.Equal(t, 0, fixture.backtestRunService.RunningCount())
	ending := fixture.lastWrite(t)
	assert.Equal(t, "failed", ending.Status)
	assert.Equal(t, "被重新啟動打斷", ending.FailureReason)
}

func TestBacktestRunServiceClaimsNothingFromAnEmptyQueue(t *testing.T) {
	testCases := []struct {
		name          string
		claimError    error
		expectedError error
	}{
		{name: "nothing queued"},
		{name: "the queue unreadable", claimError: errStorageUnavailable, expectedError: errStorageUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, buyingThenSelling)
			fixture.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), "replica-a", gomock.Any()).
				Return(entities.BacktestRun{}, false, testCase.claimError)

			claimed, err := fixture.backtestRunService.ClaimAndRun(t.Context())

			assert.False(t, claimed)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, 0, fixture.backtestRunService.RunningCount())
		})
	}
}

func TestBacktestRunServiceSweepsAndClearsOut(t *testing.T) {
	t.Run("running runs of vanished replicas fail as interrupted", func(t *testing.T) {
		fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, buyingThenSelling)
		fixture.runRepository.EXPECT().
			FailRunningOutside(gomock.Any(), []string{"replica-b"}, "被重新啟動打斷", backtestRunServiceNow).Return(2, nil)

		sweptCount, err := fixture.backtestRunService.FailInterruptedRuns(t.Context(), []string{"replica-b"})

		require.NoError(t, err)
		assert.Equal(t, 2, sweptCount)
	})

	t.Run("runs ended longer ago than the retention are deleted", func(t *testing.T) {
		fixture := newBacktestRunServiceUnderTest(t, time.Minute, 100, buyingThenSelling)
		fixture.runRepository.EXPECT().
			DeleteFinishedBefore(gomock.Any(), backtestRunServiceNow.Add(-30*24*time.Hour)).Return(1, nil)

		deletedCount, err := fixture.backtestRunService.DeleteExpiredRuns(t.Context(), 30*24*time.Hour)

		require.NoError(t, err)
		assert.Equal(t, 1, deletedCount)
	})
}
