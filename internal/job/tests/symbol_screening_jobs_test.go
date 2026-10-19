package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type screeningJobsUnderTest struct {
	application          *application.SymbolScreeningApplication
	runRepository        *mocks.MockISymbolScreeningRunRepository
	kCandleRepository    *mocks.MockISymbolScreeningKCandleRepository
	settlementRepository *mocks.MockISymbolScreeningFundingRateSettlementRepository
	looking              chan string
	endings              chan entities.SymbolScreeningRun
}

// newScreeningJobsUnderTest holds one queued run whose only candidate makes the venue wait until the run is cut off.
func newScreeningJobsUnderTest(t *testing.T) screeningJobsUnderTest {
	t.Helper()
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()
	underTest := screeningJobsUnderTest{
		runRepository:        mocks.NewMockISymbolScreeningRunRepository(controller),
		kCandleRepository:    mocks.NewMockISymbolScreeningKCandleRepository(controller),
		settlementRepository: mocks.NewMockISymbolScreeningFundingRateSettlementRepository(controller),
		looking:              make(chan string, 4),
		endings:              make(chan entities.SymbolScreeningRun, 4),
	}
	candidateRepository := mocks.NewMockISymbolScreeningCandidateRepository(controller)
	candidateRepository.EXPECT().FindByRun(gomock.Any(), gomock.Any()).Return([]entities.SymbolScreeningCandidate{
		{ID: 1, RunID: 9, Position: 1, Symbol: "LINKUSDT", Status: "waiting"},
	}, nil).AnyTimes()
	candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	symbolLookupProxy := mocks.NewMockIContractSymbolLookupProxy(controller)
	symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "LINKUSDT").
		DoAndReturn(func(executionContext context.Context, _ string) (vo.ContractSymbolListingVo, error) {
			underTest.looking <- "looking"
			<-executionContext.Done()
			return vo.ContractSymbolListingVo{}, executionContext.Err()
		}).AnyTimes()
	marginTierProxy := mocks.NewMockIContractMaintenanceMarginTierProxy(controller)
	marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(nil, nil).AnyTimes()
	underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, run entities.SymbolScreeningRun) (bool, error) {
			if run.Status != "running" {
				underTest.endings <- run
			}
			return true, nil
		}).AnyTimes()

	marketDataService := service.NewSymbolScreeningMarketDataService(
		underTest.kCandleRepository, underTest.settlementRepository, mocks.NewMockIContractMarketDataProxy(controller),
		symbolLookupProxy, mocks.NewMockIContractFundingRateProxy(controller), marginTierProxy, clockProxy)
	underTest.application = application.NewSymbolScreeningApplication(
		service.NewSymbolScreeningProfileService(mocks.NewMockISymbolScreeningProfileRepository(controller),
			underTest.runRepository, service.NewStrategyScriptService(mocks.NewMockIStrategyScriptRepository(controller),
				mocks.NewMockIPublishedStrategyScriptRepository(controller))),
		service.NewSymbolScreeningRunService(underTest.runRepository, candidateRepository, transactionRepository,
			clockProxy, marketDataService, nil, "replica-under-test", 20, 0, 0),
		marketDataService, 90*24*time.Hour, 30*24*time.Hour)

	return underTest
}

func (underTest screeningJobsUnderTest) queueHoldsOneRun() {
	underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(entities.SymbolScreeningRun{
		ID: 9, OwnerID: 7, Status: "queued", CandidateCount: 1,
		Snapshot: dto.SymbolScreeningSnapshotDto{AggregationInterval: "1d", HistoryDays: 180, SegmentCount: 3},
	}, true, nil)
	underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(entities.SymbolScreeningRun{}, false, nil).AnyTimes()
	underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
}

func (underTest screeningJobsUnderTest) lastEnding(t *testing.T) entities.SymbolScreeningRun {
	t.Helper()
	select {
	case ending := <-underTest.endings:
		return ending
	case <-time.After(2 * time.Second):
		t.Fatal("the run never ended")
		return entities.SymbolScreeningRun{}
	}
}

func TestSymbolScreeningWorkerJobScreensOnDutyAndLetsGoWhenItStops(t *testing.T) {
	underTest := newScreeningJobsUnderTest(t)
	underTest.queueHoldsOneRun()
	workerJob := job.NewSymbolScreeningRunWorkerJob(underTest.application, onDuty(t), time.Hour)
	workerJob.Start(t.Context())
	nextFrom(t, underTest.looking)

	workerJob.Stop()

	waitForFinished(t, workerJob.Finished())
	ending := underTest.lastEnding(t)
	assert.Equal(t, "failed", ending.Status)
	assert.Equal(t, "被重新啟動打斷", ending.FailureReason)
}

func TestSymbolScreeningWorkerJobLetsGoWhenItLosesDuty(t *testing.T) {
	underTest := newScreeningJobsUnderTest(t)
	underTest.queueHoldsOneRun()
	duty := newDuty(t, true)
	workerJob := job.NewSymbolScreeningRunWorkerJob(underTest.application, duty.application, 10*time.Millisecond)
	t.Cleanup(workerJob.Stop)
	workerJob.Start(t.Context())
	nextFrom(t, underTest.looking)

	duty.set(t, false)

	assert.Equal(t, "被重新啟動打斷", underTest.lastEnding(t).FailureReason)
}

func TestSymbolScreeningWorkerJobLeavesTheQueueAloneOffDuty(t *testing.T) {
	underTest := newScreeningJobsUnderTest(t)
	workerJob := job.NewSymbolScreeningRunWorkerJob(underTest.application, newDuty(t, false).application, time.Hour)

	workerJob.Start(t.Context())
	workerJob.Stop()

	waitForFinished(t, workerJob.Finished())
}

func TestSymbolScreeningWorkerJobCarriesOnWhenTheQueueCannotBeRead(t *testing.T) {
	underTest := newScreeningJobsUnderTest(t)
	read := make(chan string, 4)
	underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).
		DoAndReturn(func(context.Context) (entities.SymbolScreeningRun, bool, error) {
			read <- "read"
			return entities.SymbolScreeningRun{}, false, errors.New("storage unavailable")
		}).AnyTimes()
	workerJob := job.NewSymbolScreeningRunWorkerJob(underTest.application, onDuty(t), 10*time.Millisecond)
	t.Cleanup(workerJob.Stop)

	workerJob.Start(t.Context())

	nextFrom(t, read)
	nextFrom(t, read)
}

func TestSymbolScreeningRetentionJobClearsOutOnlyOnDuty(t *testing.T) {
	t.Run("on duty", func(t *testing.T) {
		underTest := newScreeningJobsUnderTest(t)
		deleted := make(chan string, 4)
		underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), currentTime.Add(-90*24*time.Hour)).Return(1, nil)
		underTest.kCandleRepository.EXPECT().DeleteFetchedBefore(gomock.Any(), currentTime.Add(-30*24*time.Hour)).Return(5, nil)
		underTest.settlementRepository.EXPECT().DeleteFetchedBefore(gomock.Any(), currentTime.Add(-30*24*time.Hour)).
			DoAndReturn(func(context.Context, time.Time) (int, error) {
				deleted <- "deleted"
				return 2, nil
			})
		retentionJob := job.NewSymbolScreeningRetentionJob(underTest.application, onDuty(t), time.Hour)
		t.Cleanup(retentionJob.Stop)

		retentionJob.Start(t.Context())

		assert.Equal(t, "deleted", nextFrom(t, deleted))
	})

	t.Run("on duty but the store fails", func(t *testing.T) {
		underTest := newScreeningJobsUnderTest(t)
		tried := make(chan string, 4)
		underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, time.Time) (int, error) {
				tried <- "tried"
				return 0, errors.New("storage unavailable")
			})
		retentionJob := job.NewSymbolScreeningRetentionJob(underTest.application, onDuty(t), time.Hour)
		t.Cleanup(retentionJob.Stop)

		retentionJob.Start(t.Context())

		assert.Equal(t, "tried", nextFrom(t, tried))
	})

	t.Run("off duty", func(t *testing.T) {
		underTest := newScreeningJobsUnderTest(t)
		retentionJob := job.NewSymbolScreeningRetentionJob(underTest.application, newDuty(t, false).application, time.Hour)

		retentionJob.Start(t.Context())
		retentionJob.Stop()

		waitForFinished(t, retentionJob.Finished())
	})
}
