package job_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type backtestRunJobsUnderTest struct {
	backtestRunApplication *application.BacktestRunApplication
	runRepository          *mocks.MockIBacktestRunRepository
	replaying              chan struct{}
	endingsMutex           *sync.Mutex
	endings                *[]entities.BacktestRun
}

// newBacktestRunJobsUnderTest replays runs whose scripts run until their replay is cut off.
func newBacktestRunJobsUnderTest(t *testing.T) backtestRunJobsUnderTest {
	t.Helper()
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, domains.KCandleQueryDomain, int) ([]entities.KCandle, error) {
			kCandles := []entities.KCandle{}
			for hour := range 2 {
				price := decimal.NewFromInt(100)
				kCandles = append(kCandles, entities.KCandle{
					Symbol: "BTCUSDT", OpenTime: currentTime.Add(time.Duration(hour-5) * time.Hour),
					Open: price, High: price, Low: price, Close: price,
				})
			}
			return kCandles, nil
		}).AnyTimes()
	replaying := make(chan struct{}, 8)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			_ []vo.KCandleVo, _ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
		) ([]map[string]vo.IndicatorValueVo, error) {
			replaying <- struct{}{}
			<-executionContext.Done()
			return nil, errors.New("cut off")
		}).AnyTimes()
	endingsMutex := &sync.Mutex{}
	endings := &[]entities.BacktestRun{}
	runRepository := mocks.NewMockIBacktestRunRepository(controller)
	runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, write entities.BacktestRun) (bool, error) {
			endingsMutex.Lock()
			defer endingsMutex.Unlock()
			*endings = append(*endings, write)
			return true, nil
		}).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()

	return backtestRunJobsUnderTest{
		backtestRunApplication: application.NewBacktestRunApplication(service.NewBacktestRunService(
			runRepository, mocks.NewMockIBacktestRunResultRepository(controller), transactionRepository, clockProxy,
			service.NewBacktestService(kCandleRepository, indicatorScriptProxy, clockProxy, 1000, 0), nil,
			domains.NewBacktestRunAdmissionDomain(3), "replica-a", time.Minute, 100), 2, 30*24*time.Hour),
		runRepository: runRepository,
		replaying:     replaying,
		endingsMutex:  endingsMutex,
		endings:       endings,
	}
}

func (underTest backtestRunJobsUnderTest) queueHoldsOneRun() {
	requestDto := dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: currentTime.Add(-5 * time.Hour), EndTime: currentTime.Add(-2 * time.Hour),
		Script: "the script", InitialCapital: decimal.NewFromInt(10000), PositionSizingMode: "allIn",
	}
	underTest.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), "replica-a", gomock.Any()).
		Return(entities.BacktestRun{
			ID: 27, Kind: string(vo.BacktestRunSpotScript), Status: string(vo.BacktestRunRunning),
			ReplicaName: "replica-a", Request: dto.BacktestRunRequestDto{SpotScript: &requestDto},
		}, true, nil)
	underTest.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), "replica-a", gomock.Any()).
		Return(entities.BacktestRun{}, false, nil).AnyTimes()
}

func (underTest backtestRunJobsUnderTest) lastEnding(t *testing.T) entities.BacktestRun {
	t.Helper()
	underTest.endingsMutex.Lock()
	defer underTest.endingsMutex.Unlock()
	require.NotEmpty(t, *underTest.endings)

	return (*underTest.endings)[len(*underTest.endings)-1]
}

func waitForReplaying(t *testing.T, replaying chan struct{}) {
	t.Helper()
	select {
	case <-replaying:
	case <-time.After(2 * time.Second):
		t.Fatal("no run was taken off the queue")
	}
}

func waitForFinished(t *testing.T, finished <-chan struct{}) {
	t.Helper()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("the job did not finish")
	}
}

func TestBacktestRunWorkerJobLetsGoOfItsRunsWhenItStops(t *testing.T) {
	testCases := []struct {
		name string
		stop func(workerJob *job.BacktestRunWorkerJob, cancel context.CancelFunc)
	}{
		{name: "a graceful stop", stop: func(workerJob *job.BacktestRunWorkerJob, _ context.CancelFunc) { workerJob.Stop() }},
		{name: "the work being given up", stop: func(_ *job.BacktestRunWorkerJob, cancel context.CancelFunc) { cancel() }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newBacktestRunJobsUnderTest(t)
			underTest.queueHoldsOneRun()
			workerJob := job.NewBacktestRunWorkerJob(underTest.backtestRunApplication, time.Hour)
			jobContext, cancel := context.WithCancel(t.Context())
			defer cancel()
			workerJob.Start(jobContext)
			waitForReplaying(t, underTest.replaying)

			testCase.stop(workerJob, cancel)

			waitForFinished(t, workerJob.Finished())
			ending := underTest.lastEnding(t)
			assert.Equal(t, "failed", ending.Status)
			assert.Equal(t, "被重新啟動打斷", ending.FailureReason)
		})
	}
}

func TestBacktestRunRetentionJobClearsOutOnlyOnDuty(t *testing.T) {
	t.Run("on duty", func(t *testing.T) {
		underTest := newBacktestRunJobsUnderTest(t)
		deleted := make(chan string, 4)
		underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), currentTime.Add(-30*24*time.Hour)).
			DoAndReturn(func(context.Context, time.Time) (int, error) {
				deleted <- "deleted"
				return 2, nil
			})
		retentionJob := job.NewBacktestRunRetentionJob(underTest.backtestRunApplication, onDuty(t), time.Hour)
		t.Cleanup(retentionJob.Stop)

		retentionJob.Start(t.Context())

		assert.Equal(t, "deleted", nextFrom(t, deleted))
	})

	t.Run("off duty", func(t *testing.T) {
		underTest := newBacktestRunJobsUnderTest(t)
		duty := newDuty(t, false)
		retentionJob := job.NewBacktestRunRetentionJob(underTest.backtestRunApplication, duty.application, time.Hour)

		retentionJob.Start(t.Context())
		retentionJob.Stop()

		waitForFinished(t, retentionJob.Finished())
	})
}
