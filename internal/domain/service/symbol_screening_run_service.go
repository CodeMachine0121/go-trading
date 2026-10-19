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
)

const (
	symbolScreeningRunListDefaultLimit = 20
	symbolScreeningRunListMaximumLimit = 200
	symbolScreeningRunWaitMaximum      = 60 * time.Second
	symbolScreeningRunWaitPoll         = time.Second
)

type SymbolScreeningRunService struct {
	runRepository           domaininterface.ISymbolScreeningRunRepository
	candidateRepository     domaininterface.ISymbolScreeningCandidateRepository
	transactionRepository   domaininterface.ITransactionRepository
	clockProxy              domaininterface.IClockProxy
	marketDataService       *SymbolScreeningMarketDataService
	contractBacktestService *ContractBacktestService
	replicaName             string
	maximumCandidateCount   int
	timeAllowance           time.Duration
	defaultWarmupBarCount   int
	running                 atomic.Pointer[symbolScreeningRunner]
	runnerEnded             sync.WaitGroup
}

func NewSymbolScreeningRunService(
	runRepository domaininterface.ISymbolScreeningRunRepository,
	candidateRepository domaininterface.ISymbolScreeningCandidateRepository,
	transactionRepository domaininterface.ITransactionRepository,
	clockProxy domaininterface.IClockProxy,
	marketDataService *SymbolScreeningMarketDataService,
	contractBacktestService *ContractBacktestService,
	replicaName string,
	maximumCandidateCount int,
	timeAllowance time.Duration,
	defaultWarmupBarCount int,
) *SymbolScreeningRunService {
	return &SymbolScreeningRunService{
		runRepository:           runRepository,
		candidateRepository:     candidateRepository,
		transactionRepository:   transactionRepository,
		clockProxy:              clockProxy,
		marketDataService:       marketDataService,
		contractBacktestService: contractBacktestService,
		replicaName:             replicaName,
		maximumCandidateCount:   max(maximumCandidateCount, 1),
		timeAllowance:           timeAllowance,
		defaultWarmupBarCount:   defaultWarmupBarCount,
	}
}

func (runService *SymbolScreeningRunService) Submit(
	executionContext context.Context,
	ownerID uint,
	profileID uint,
	snapshot dto.SymbolScreeningSnapshotDto,
	symbolList string,
) (dto.SymbolScreeningRunDto, error) {
	symbols, listError := domains.NewSymbolScreeningSymbolListDomain(symbolList, runService.maximumCandidateCount, false)
	if listError != nil {
		return dto.SymbolScreeningRunDto{}, listError
	}
	submitted := domains.NewSubmittedSymbolScreeningRunDomain(
		ownerID, profileID, snapshot, symbols.Symbols(), runService.clockProxy.Now())

	saved := entities.SymbolScreeningRun{}
	transactionError := runService.transactionRepository.Atomically(executionContext,
		func(transactionContext context.Context) error {
			activeCount, countError := runService.runRepository.CountActiveByOwner(transactionContext, ownerID)
			if countError != nil {
				return countError
			}
			if admitError := domains.NewSymbolScreeningAdmissionDomain().Admit(activeCount); admitError != nil {
				return admitError
			}

			savedRun, saveError := runService.runRepository.Save(transactionContext, submitted.ToEntity())
			saved = savedRun

			return saveError
		})
	if transactionError != nil {
		return dto.SymbolScreeningRunDto{}, transactionError
	}

	return domains.NewSymbolScreeningRunDomain(saved).ToDto(), nil
}

func (runService *SymbolScreeningRunService) List(
	executionContext context.Context, ownerID uint, query dto.SymbolScreeningRunListQueryDto,
) (dto.SymbolScreeningRunPageDto, error) {
	if query.Limit <= 0 {
		query.Limit = symbolScreeningRunListDefaultLimit
	}
	query.Limit = min(query.Limit, symbolScreeningRunListMaximumLimit)

	runs, totalCount, findError := runService.runRepository.FindPageByOwner(executionContext, ownerID, query)
	if findError != nil {
		return dto.SymbolScreeningRunPageDto{}, findError
	}

	runDtos := make([]dto.SymbolScreeningRunDto, 0, len(runs))
	for _, run := range runs {
		runDtos = append(runDtos, domains.NewSymbolScreeningRunDomain(run).ToDto())
	}

	return dto.SymbolScreeningRunPageDto{Runs: runDtos, TotalCount: totalCount}, nil
}

// Get answers at once, or, given a wait, keeps looking once a second until the run ends or the wait is spent.
func (runService *SymbolScreeningRunService) Get(
	executionContext context.Context, ownerID uint, id uint, wait time.Duration,
) (dto.SymbolScreeningRunDetailDto, error) {
	deadline := runService.clockProxy.Now().Add(min(max(wait, 0), symbolScreeningRunWaitMaximum))

	for {
		run, found, findError := runService.runRepository.FindOneOwned(executionContext, ownerID, id)
		if findError != nil {
			return dto.SymbolScreeningRunDetailDto{}, findError
		}
		if !found {
			return dto.SymbolScreeningRunDetailDto{}, domains.ErrSymbolScreeningRunNotFound
		}

		runDomain := domains.NewSymbolScreeningRunDomain(run)
		if runDomain.IsActive() && runService.clockProxy.Now().Before(deadline) && executionContext.Err() == nil {
			runService.clockProxy.Sleep(symbolScreeningRunWaitPoll)
			continue
		}

		candidates, candidatesError := runService.candidateRepository.FindByRun(executionContext, run.ID)
		if candidatesError != nil {
			return dto.SymbolScreeningRunDetailDto{}, candidatesError
		}

		return runDomain.ToDetailDto(candidates), nil
	}
}

// Cancel ends a queued or running run; a runner on this replica stops at once, one elsewhere at its next candidate.
func (runService *SymbolScreeningRunService) Cancel(
	executionContext context.Context, ownerID uint, id uint,
) (dto.SymbolScreeningRunDto, error) {
	run, found, findError := runService.runRepository.FindOneOwned(executionContext, ownerID, id)
	if findError != nil {
		return dto.SymbolScreeningRunDto{}, findError
	}
	if !found {
		return dto.SymbolScreeningRunDto{}, domains.ErrSymbolScreeningRunNotFound
	}

	cancelled, cancelError := domains.NewSymbolScreeningRunDomain(run).Cancelled(runService.clockProxy.Now())
	if cancelError != nil {
		return dto.SymbolScreeningRunDto{}, cancelError
	}

	transactionError := runService.transactionRepository.Atomically(executionContext,
		func(transactionContext context.Context) error {
			wasActive, writeError := runService.runRepository.CancelIfActive(transactionContext, cancelled.ToEntity())
			if writeError != nil {
				return writeError
			}
			if !wasActive {
				return domains.ErrSymbolScreeningRunAlreadyEnded
			}

			return runService.candidateRepository.CancelUnfinished(transactionContext, run.ID)
		})
	if transactionError != nil {
		return dto.SymbolScreeningRunDto{}, transactionError
	}
	if runner := runService.running.Load(); runner != nil && runner.runID == id {
		runner.cancel()
	}

	return cancelled.ToDto(), nil
}

// ClaimAndRun takes the oldest queued run when this replica is screening nothing.
func (runService *SymbolScreeningRunService) ClaimAndRun(executionContext context.Context) (bool, error) {
	if runService.running.Load() != nil {
		return false, nil
	}

	claimed := domains.SymbolScreeningRunDomain{}
	wasClaimed := false
	claimError := runService.transactionRepository.Atomically(executionContext,
		func(transactionContext context.Context) error {
			queued, found, lockError := runService.runRepository.LockOldestQueued(transactionContext)
			if lockError != nil || !found {
				return lockError
			}

			claimed = domains.NewSymbolScreeningRunDomain(queued).Claimed(runService.replicaName, runService.clockProxy.Now())
			wasClaimed = true

			return runService.runRepository.SaveClaim(transactionContext, claimed.ToEntity())
		})
	if claimError != nil || !wasClaimed {
		return false, claimError
	}

	runner := newSymbolScreeningRunner(runService, claimed)
	runService.running.Store(runner)
	runService.runnerEnded.Add(1)
	go runner.run()

	return true, nil
}

// StopRunner cuts off the run this replica is screening and returns once it is marked interrupted.
func (runService *SymbolScreeningRunService) StopRunner() {
	if runner := runService.running.Load(); runner != nil {
		runner.interrupt()
	}
	runService.runnerEnded.Wait()
}

func (runService *SymbolScreeningRunService) FailInterruptedRuns(
	executionContext context.Context, liveReplicaNames []string,
) (int, error) {
	return runService.runRepository.FailRunningOutside(
		executionContext, liveReplicaNames, domains.SymbolScreeningInterruptedReason, runService.clockProxy.Now())
}

func (runService *SymbolScreeningRunService) DeleteExpiredRuns(
	executionContext context.Context, retention time.Duration,
) (int, error) {
	return runService.runRepository.DeleteFinishedBefore(executionContext, runService.clockProxy.Now().Add(-retention))
}

func (runService *SymbolScreeningRunService) runnerFinished() {
	runService.running.Store(nil)
	runService.runnerEnded.Done()
}
