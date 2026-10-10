package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	backtestRunListDefaultLimit = 20
	backtestRunListMaximumLimit = 200
	// backtestRunWaitMaximum caps how long one read may hang on; the poll is one second so a finish is seen about that late at most.
	backtestRunWaitMaximum = 60 * time.Second
	backtestRunWaitPoll    = time.Second
)

// BacktestRunService owns a run from submission to ending: the queue, the per-owner cap, claiming on this replica, cancelling, sweeping and clearing out; the replay itself is the replay services' business.
type BacktestRunService struct {
	backtestRunRepository       domaininterface.IBacktestRunRepository
	backtestRunResultRepository domaininterface.IBacktestRunResultRepository
	transactionRepository       domaininterface.ITransactionRepository
	clockProxy                  domaininterface.IClockProxy
	backtestService             *BacktestService
	contractBacktestService     *ContractBacktestService
	admission                   domains.BacktestRunAdmissionDomain
	replicaName                 string
	timeAllowance               time.Duration
	progressWriteEveryBars      int
	// runningCount is how many runners this replica has in flight, the figure the worker job compares with its cap.
	runningCount atomic.Int32
	runners      sync.Map
	// runnersEnded lets a graceful stop wait until every runner has written its ending.
	runnersEnded sync.WaitGroup
}

func NewBacktestRunService(
	backtestRunRepository domaininterface.IBacktestRunRepository,
	backtestRunResultRepository domaininterface.IBacktestRunResultRepository,
	transactionRepository domaininterface.ITransactionRepository,
	clockProxy domaininterface.IClockProxy,
	backtestService *BacktestService,
	contractBacktestService *ContractBacktestService,
	admission domains.BacktestRunAdmissionDomain,
	replicaName string,
	timeAllowance time.Duration,
	progressWriteEveryBars int,
) *BacktestRunService {
	return &BacktestRunService{
		backtestRunRepository:       backtestRunRepository,
		backtestRunResultRepository: backtestRunResultRepository,
		transactionRepository:       transactionRepository,
		clockProxy:                  clockProxy,
		backtestService:             backtestService,
		contractBacktestService:     contractBacktestService,
		admission:                   admission,
		replicaName:                 replicaName,
		timeAllowance:               timeAllowance,
		progressWriteEveryBars:      max(progressWriteEveryBars, 1),
	}
}

// Submit records a queued run once the owner is under the cap; the request is already resolved and validated by the caller, so nothing invalid is ever queued.
func (backtestRunService *BacktestRunService) Submit(
	executionContext context.Context, ownerID uint, submission dto.BacktestRunSubmissionDto,
) (dto.BacktestRunDto, error) {
	submitted, describeError := domains.NewSubmittedBacktestRunDomain(
		ownerID, submission, backtestRunService.clockProxy.Now())
	if describeError != nil {
		return dto.BacktestRunDto{}, describeError
	}

	saved := entities.BacktestRun{}
	transactionError := backtestRunService.transactionRepository.Atomically(executionContext,
		func(transactionContext context.Context) error {
			activeCount, countError := backtestRunService.backtestRunRepository.CountActiveByOwner(
				transactionContext, ownerID)
			if countError != nil {
				return countError
			}
			if admitError := backtestRunService.admission.Admit(activeCount); admitError != nil {
				return admitError
			}

			savedRun, saveError := backtestRunService.backtestRunRepository.Save(
				transactionContext, submitted.ToEntity())
			saved = savedRun

			return saveError
		})
	if transactionError != nil {
		return dto.BacktestRunDto{}, transactionError
	}

	return domains.NewBacktestRunDomain(saved).ToDto(), nil
}

// List is newest first and never carries a report.
func (backtestRunService *BacktestRunService) List(
	executionContext context.Context, ownerID uint, query dto.BacktestRunListQueryDto,
) (dto.BacktestRunPageDto, error) {
	if query.Limit <= 0 {
		query.Limit = backtestRunListDefaultLimit
	}
	query.Limit = min(query.Limit, backtestRunListMaximumLimit)

	backtestRuns, totalCount, findError := backtestRunService.backtestRunRepository.FindPageByOwner(
		executionContext, ownerID, query)
	if findError != nil {
		return dto.BacktestRunPageDto{}, findError
	}

	runDtos := make([]dto.BacktestRunDto, 0, len(backtestRuns))
	for _, backtestRun := range backtestRuns {
		runDtos = append(runDtos, domains.NewBacktestRunDomain(backtestRun).ToDto())
	}

	return dto.BacktestRunPageDto{Runs: runDtos, TotalCount: totalCount}, nil
}

// Get answers at once, or, given a wait, keeps looking once a second until the run ends or the wait is spent; a report is attached only to a succeeded run.
func (backtestRunService *BacktestRunService) Get(
	executionContext context.Context, ownerID uint, id uint, wait time.Duration,
) (dto.BacktestRunDetailDto, error) {
	deadline := backtestRunService.clockProxy.Now().Add(min(max(wait, 0), backtestRunWaitMaximum))

	for {
		backtestRun, found, findError := backtestRunService.backtestRunRepository.FindOneOwned(
			executionContext, ownerID, id)
		if findError != nil {
			return dto.BacktestRunDetailDto{}, findError
		}
		if !found {
			return dto.BacktestRunDetailDto{}, domains.ErrBacktestRunNotFound
		}

		runDomain := domains.NewBacktestRunDomain(backtestRun)
		if runDomain.IsActive() && backtestRunService.clockProxy.Now().Before(deadline) &&
			executionContext.Err() == nil {
			backtestRunService.clockProxy.Sleep(backtestRunWaitPoll)
			continue
		}

		if runDomain.IsActive() || backtestRun.Status != string(vo.BacktestRunSucceeded) {
			return runDomain.ToDetailDto(nil), nil
		}

		result, hasResult, resultError := backtestRunService.backtestRunResultRepository.FindOne(
			executionContext, backtestRun.ID)
		if resultError != nil {
			return dto.BacktestRunDetailDto{}, resultError
		}
		if !hasResult {
			return runDomain.ToDetailDto(nil), nil
		}
		resultDto := result.ToDto()

		return runDomain.ToDetailDto(&resultDto), nil
	}
}

// Cancel ends a queued or running run of this owner; a running one is stopped by its runner at its next progress write.
func (backtestRunService *BacktestRunService) Cancel(
	executionContext context.Context, ownerID uint, id uint,
) (dto.BacktestRunDto, error) {
	backtestRun, found, findError := backtestRunService.backtestRunRepository.FindOneOwned(
		executionContext, ownerID, id)
	if findError != nil {
		return dto.BacktestRunDto{}, findError
	}
	if !found {
		return dto.BacktestRunDto{}, domains.ErrBacktestRunNotFound
	}

	cancelled, cancelError := domains.NewBacktestRunDomain(backtestRun).Cancelled(
		backtestRunService.clockProxy.Now())
	if cancelError != nil {
		return dto.BacktestRunDto{}, cancelError
	}

	wasActive, writeError := backtestRunService.backtestRunRepository.CancelIfActive(
		executionContext, cancelled.ToEntity())
	if writeError != nil {
		return dto.BacktestRunDto{}, writeError
	}
	// Ended between the read and the write, which is the same refusal as reading it ended.
	if !wasActive {
		return dto.BacktestRunDto{}, domains.ErrBacktestRunAlreadyEnded
	}

	return cancelled.ToDto(), nil
}

// ClaimAndRun takes the oldest queued run, if any, and replays it in the background under this replica's name.
func (backtestRunService *BacktestRunService) ClaimAndRun(executionContext context.Context) (bool, error) {
	claimed, wasClaimed, claimError := backtestRunService.backtestRunRepository.ClaimOldestQueued(
		executionContext, backtestRunService.replicaName, backtestRunService.clockProxy.Now())
	if claimError != nil || !wasClaimed {
		return false, claimError
	}

	runner := newBacktestRunner(backtestRunService, domains.NewBacktestRunDomain(claimed))
	backtestRunService.runningCount.Add(1)
	backtestRunService.runnersEnded.Add(1)
	backtestRunService.runners.Store(claimed.ID, runner)
	go runner.run()

	return true, nil
}

// RunningCount is how many runs this replica is replaying right now.
func (backtestRunService *BacktestRunService) RunningCount() int {
	return int(backtestRunService.runningCount.Load())
}

// StopRunners cuts off every run this replica is replaying and returns once each is marked interrupted, so a graceful stop leaves nothing looking in flight; it must not race a claim.
func (backtestRunService *BacktestRunService) StopRunners() {
	backtestRunService.runners.Range(func(_ any, value any) bool {
		value.(*backtestRunner).interrupt()

		return true
	})
	backtestRunService.runnersEnded.Wait()
}

// FailInterruptedRuns fails runs left running by replicas no longer alive; queued runs are untouched, since any replica can still take them.
func (backtestRunService *BacktestRunService) FailInterruptedRuns(
	executionContext context.Context, liveReplicaNames []string,
) (int, error) {
	return backtestRunService.backtestRunRepository.FailRunningOutside(
		executionContext, liveReplicaNames, domains.BacktestRunInterruptedReason,
		backtestRunService.clockProxy.Now())
}

// DeleteExpiredRuns clears runs that ended longer ago than the retention, reports included.
func (backtestRunService *BacktestRunService) DeleteExpiredRuns(
	executionContext context.Context, retention time.Duration,
) (int, error) {
	return backtestRunService.backtestRunRepository.DeleteFinishedBefore(
		executionContext, backtestRunService.clockProxy.Now().Add(-retention))
}

func (backtestRunService *BacktestRunService) runnerEnded(id uint) {
	backtestRunService.runners.Delete(id)
	backtestRunService.runningCount.Add(-1)
	backtestRunService.runnersEnded.Done()
}
