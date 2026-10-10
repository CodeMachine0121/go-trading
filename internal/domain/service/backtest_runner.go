package service

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	backtestRunBrokeDown     = "回測服務壞掉了，這筆沒有跑完"
	backtestRunOverAllowance = "超過時間上限"
	// backtestRunWriteTimeout keeps a stuck database from holding a runner, and with it a graceful stop, forever.
	backtestRunWriteTimeout = 10 * time.Second
)

type backtestRunner struct {
	backtestRunService *BacktestRunService
	backtestRunID      uint
	ownerID            uint
	kind               vo.BacktestRunKindVo
	request            dto.BacktestRunRequestDto
	// replayContext is not derived from any request, so the replay outlives the submit.
	replayContext context.Context
	stopReplaying context.CancelFunc
	// progressMutex guards the run's state, which the proxy's reader goroutine may still be advancing after a cancel or a timeout.
	progressMutex   sync.Mutex
	backtestRun     domains.BacktestRunDomain
	writtenBarCount int
	ended           bool
	interrupted     bool
	cancelled       bool
}

func newBacktestRunner(backtestRunService *BacktestRunService, run domains.BacktestRunDomain) *backtestRunner {
	replayContext, stopReplaying := context.WithCancel(context.Background())
	claimed := run.ToEntity()

	return &backtestRunner{
		backtestRunService: backtestRunService,
		backtestRunID:      claimed.ID,
		ownerID:            claimed.OwnerID,
		kind:               run.Kind(),
		request:            run.Request(),
		replayContext:      replayContext,
		stopReplaying:      stopReplaying,
		backtestRun:        run,
	}
}

func (backtestRunner *backtestRunner) run() {
	service := backtestRunner.backtestRunService
	defer service.runnerEnded(backtestRunner.backtestRunID)
	defer backtestRunner.stopReplaying()
	defer func() {
		if panicValue := recover(); panicValue != nil {
			log.Printf("backtest run %d panicked: %v\n%s", backtestRunner.backtestRunID, panicValue, debug.Stack())
			backtestRunner.end(errBacktestRunBrokeDown, dto.BacktestRunSummaryDto{}, nil)
		}
	}()
	go backtestRunner.watchForCancellation()

	switch backtestRunner.kind {
	case vo.BacktestRunSpotScript:
		requestDto := *backtestRunner.request.SpotScript
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.backtestService.RunBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)
		backtestRunner.end(replayError, resultDto.ToBacktestRunSummaryDto(),
			&entities.BacktestRunResult{BacktestRunID: backtestRunner.backtestRunID, SpotResult: &resultDto})
	case vo.BacktestRunSpotTradingStrategy:
		requestDto := *backtestRunner.request.SpotTradingStrategy
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.backtestService.RunTradingStrategyBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)
		backtestRunner.end(replayError, resultDto.ToBacktestRunSummaryDto(),
			&entities.BacktestRunResult{BacktestRunID: backtestRunner.backtestRunID, SpotResult: &resultDto})
	case vo.BacktestRunContractScript:
		requestDto := *backtestRunner.request.ContractScript
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.contractBacktestService.RunContractBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)
		backtestRunner.end(replayError, resultDto.ToBacktestRunSummaryDto(),
			&entities.BacktestRunResult{BacktestRunID: backtestRunner.backtestRunID, ContractResult: &resultDto})
	case vo.BacktestRunContractTradingStrategy:
		requestDto := *backtestRunner.request.ContractTradingStrategy
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.contractBacktestService.RunContractTradingStrategyBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)
		backtestRunner.end(replayError, resultDto.ToBacktestRunSummaryDto(),
			&entities.BacktestRunResult{BacktestRunID: backtestRunner.backtestRunID, ContractResult: &resultDto})
	default:
		backtestRunner.end(errors.New("認不得的回測種類，這筆沒有跑"), dto.BacktestRunSummaryDto{}, nil)
	}
}

// watchForCancellation re-reads the run now and then, so a cancel from any replica stops a replay that is waiting for a compartment or reading candles, not only one that is writing progress.
func (backtestRunner *backtestRunner) watchForCancellation() {
	service := backtestRunner.backtestRunService
	statusCheck := time.NewTicker(service.statusCheckInterval)
	defer statusCheck.Stop()

	for {
		select {
		case <-backtestRunner.replayContext.Done():
			return
		case <-statusCheck.C:
		}

		readContext, endRead := context.WithTimeout(backtestRunner.replayContext, backtestRunWriteTimeout)
		stored, found, readError := service.backtestRunRepository.FindOneOwned(
			readContext, backtestRunner.ownerID, backtestRunner.backtestRunID)
		endRead()
		if readError != nil {
			if backtestRunner.replayContext.Err() == nil {
				log.Printf("backtest run %d could not be checked for a cancel: %v", backtestRunner.backtestRunID, readError)
			}
			continue
		}
		if !found || stored.Status != string(vo.BacktestRunRunning) || stored.ReplicaName != service.replicaName {
			backtestRunner.cancel()
			return
		}
	}
}

// ReplayAdvanced lands progress every so many bars; a refused write means the run is no longer running (cancelled or swept), so the replay is stopped.
func (backtestRunner *backtestRunner) ReplayAdvanced(completedBarCount int, totalBarCount int) {
	backtestRunner.progressMutex.Lock()
	defer backtestRunner.progressMutex.Unlock()

	if backtestRunner.ended || backtestRunner.cancelled {
		return
	}
	backtestRunner.backtestRun = backtestRunner.backtestRun.Advanced(completedBarCount, totalBarCount)
	if completedBarCount-backtestRunner.writtenBarCount < backtestRunner.backtestRunService.progressWriteEveryBars &&
		completedBarCount != totalBarCount {
		return
	}
	backtestRunner.writtenBarCount = completedBarCount

	writeContext, endWrite := context.WithTimeout(context.Background(), backtestRunWriteTimeout)
	defer endWrite()
	stillRunning, writeError := backtestRunner.backtestRunService.backtestRunRepository.UpdateIfRunning(
		writeContext, backtestRunner.backtestRun.ToEntity())
	if writeError != nil {
		log.Printf("backtest run %d progress could not be written: %v", backtestRunner.backtestRunID, writeError)

		return
	}
	if !stillRunning {
		backtestRunner.cancelled = true
		backtestRunner.stopReplaying()
	}
}

// cancel stops a run that has already been ended elsewhere, so nothing more is written for it.
func (backtestRunner *backtestRunner) cancel() {
	backtestRunner.progressMutex.Lock()
	backtestRunner.cancelled = true
	backtestRunner.progressMutex.Unlock()

	backtestRunner.stopReplaying()
}

// interrupt is a graceful stop: the replay is cut off and the ending reads as a restart.
func (backtestRunner *backtestRunner) interrupt() {
	backtestRunner.progressMutex.Lock()
	backtestRunner.interrupted = true
	backtestRunner.progressMutex.Unlock()

	backtestRunner.stopReplaying()
}

// end moves the run from whatever state it reached to its ending under the lock, then writes it with, for a success, the report in the same transaction; a refused ending means the run was cancelled or swept meanwhile and the report is dropped.
func (backtestRunner *backtestRunner) end(
	replayError error, summary dto.BacktestRunSummaryDto, result *entities.BacktestRunResult,
) {
	backtestRunner.progressMutex.Lock()
	defer backtestRunner.progressMutex.Unlock()

	if backtestRunner.ended {
		return
	}
	backtestRunner.ended = true
	if backtestRunner.cancelled {
		return
	}

	service := backtestRunner.backtestRunService
	now := service.clockProxy.Now()
	switch {
	case replayError == nil:
		backtestRunner.backtestRun = backtestRunner.backtestRun.Succeeded(summary, now)
	case backtestRunner.interrupted:
		backtestRunner.backtestRun = backtestRunner.backtestRun.Failed(domains.BacktestRunInterruptedReason, now)
	case errors.Is(replayError, errBacktestRunBrokeDown):
		backtestRunner.backtestRun = backtestRunner.backtestRun.Failed(backtestRunBrokeDown, now)
	case errors.Is(replayError, domains.ErrBacktestTimeAllowanceSpent):
		backtestRunner.backtestRun = backtestRunner.backtestRun.Failed(backtestRunOverAllowance, now)
	default:
		backtestRunner.backtestRun = backtestRunner.backtestRun.Failed(replayError.Error(), now)
	}
	if replayError != nil {
		result = nil
	}
	ending := backtestRunner.backtestRun.ToEntity()

	writeContext, endWrite := context.WithTimeout(context.Background(), backtestRunWriteTimeout)
	defer endWrite()
	writeError := service.transactionRepository.Atomically(writeContext,
		func(transactionContext context.Context) error {
			stillRunning, updateError := service.backtestRunRepository.UpdateIfRunning(transactionContext, ending)
			if updateError != nil || !stillRunning || result == nil {
				return updateError
			}

			return service.backtestRunResultRepository.Save(transactionContext, *result)
		})
	if writeError != nil {
		log.Printf("backtest run %d ending could not be written: %v", backtestRunner.backtestRunID, writeError)
	}
}

var errBacktestRunBrokeDown = errors.New("backtest run broke down")
