package service

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"sync"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	backtestRunBrokeDown     = "回測服務壞掉了，這筆沒有跑完"
	backtestRunOverAllowance = "超過時間上限"
)

// backtestRunner replays one claimed run in the background and writes its progress and ending; it holds execution state only, the rules live in BacktestRunDomain.
type backtestRunner struct {
	backtestRunService *BacktestRunService
	backtestRun        domains.BacktestRunDomain
	// replayContext is not derived from any request, so the replay outlives the submit; the run's allowance travels in its request.
	replayContext context.Context
	stopReplaying context.CancelFunc
	// progressMutex serialises the proxy's reader goroutine with the ending, so a late progress write cannot follow the closing one.
	progressMutex   sync.Mutex
	writtenBarCount int
	ended           bool
	// interrupted is set by a graceful stop, so the ending reads as a restart rather than a cancel.
	interrupted bool
}

func newBacktestRunner(backtestRunService *BacktestRunService, run domains.BacktestRunDomain) *backtestRunner {
	replayContext, stopReplaying := context.WithCancel(context.Background())

	return &backtestRunner{
		backtestRunService: backtestRunService,
		backtestRun:        run,
		replayContext:      replayContext,
		stopReplaying:      stopReplaying,
	}
}

func (backtestRunner *backtestRunner) run() {
	service := backtestRunner.backtestRunService
	defer service.runnerEnded(backtestRunner.backtestRun.ToEntity().ID)
	defer backtestRunner.stopReplaying()
	defer func() {
		if panicValue := recover(); panicValue != nil {
			log.Printf("backtest run %d panicked: %v\n%s",
				backtestRunner.backtestRun.ToEntity().ID, panicValue, debug.Stack())
			backtestRunner.end(backtestRunner.backtestRun.Failed(backtestRunBrokeDown, service.clockProxy.Now()), nil)
		}
	}()

	result, summary, replayError := backtestRunner.replay()
	now := service.clockProxy.Now()
	switch {
	case replayError == nil:
		backtestRunner.end(backtestRunner.backtestRun.Succeeded(summary, now), &result)
	case backtestRunner.wasInterrupted():
		backtestRunner.end(backtestRunner.backtestRun.Failed(domains.BacktestRunInterruptedReason, now), nil)
	case errors.Is(replayError, domains.ErrBacktestTimeAllowanceSpent):
		backtestRunner.end(backtestRunner.backtestRun.Failed(backtestRunOverAllowance, now), nil)
	default:
		backtestRunner.end(backtestRunner.backtestRun.Failed(replayError.Error(), now), nil)
	}
}

// replay dispatches on the run's kind under the background allowance; each report also yields the headline figures a list shows.
func (backtestRunner *backtestRunner) replay() (entities.BacktestRunResult, dto.BacktestRunSummaryDto, error) {
	request := backtestRunner.backtestRun.Request()
	runID := backtestRunner.backtestRun.ToEntity().ID
	service := backtestRunner.backtestRunService

	switch backtestRunner.backtestRun.Kind() {
	case vo.BacktestRunSpotScript:
		requestDto := *request.SpotScript
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.backtestService.RunBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)

		return entities.BacktestRunResult{BacktestRunID: runID, SpotResult: &resultDto},
			resultDto.ToBacktestRunSummaryDto(), replayError
	case vo.BacktestRunSpotTradingStrategy:
		requestDto := *request.SpotTradingStrategy
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.backtestService.RunTradingStrategyBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)

		return entities.BacktestRunResult{BacktestRunID: runID, SpotResult: &resultDto},
			resultDto.ToBacktestRunSummaryDto(), replayError
	case vo.BacktestRunContractScript:
		requestDto := *request.ContractScript
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.contractBacktestService.RunContractBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)

		return entities.BacktestRunResult{BacktestRunID: runID, ContractResult: &resultDto},
			resultDto.ToBacktestRunSummaryDto(), replayError
	case vo.BacktestRunContractTradingStrategy:
		requestDto := *request.ContractTradingStrategy
		requestDto.TimeAllowance = service.timeAllowance
		resultDto, replayError := service.contractBacktestService.RunContractTradingStrategyBacktest(
			backtestRunner.replayContext, requestDto, backtestRunner)

		return entities.BacktestRunResult{BacktestRunID: runID, ContractResult: &resultDto},
			resultDto.ToBacktestRunSummaryDto(), replayError
	default:
		return entities.BacktestRunResult{}, dto.BacktestRunSummaryDto{},
			errors.New("認不得的回測種類，這筆沒有跑")
	}
}

// ReplayAdvanced lands progress every so many bars; a refused write means the run is no longer running (cancelled or swept), so the replay is stopped.
func (backtestRunner *backtestRunner) ReplayAdvanced(completedBarCount int, totalBarCount int) {
	backtestRunner.progressMutex.Lock()
	defer backtestRunner.progressMutex.Unlock()

	if backtestRunner.ended {
		return
	}
	backtestRunner.backtestRun = backtestRunner.backtestRun.Advanced(completedBarCount, totalBarCount)
	if completedBarCount-backtestRunner.writtenBarCount < backtestRunner.backtestRunService.progressWriteEveryBars &&
		completedBarCount != totalBarCount {
		return
	}
	backtestRunner.writtenBarCount = completedBarCount

	stillRunning, writeError := backtestRunner.backtestRunService.backtestRunRepository.UpdateIfRunning(
		context.Background(), backtestRunner.backtestRun.ToEntity())
	if writeError != nil {
		log.Printf("backtest run %d progress could not be written: %v",
			backtestRunner.backtestRun.ToEntity().ID, writeError)

		return
	}
	if !stillRunning {
		backtestRunner.stopReplaying()
	}
}

// interrupt is a graceful stop: the replay is cut off and the ending reads as a restart.
func (backtestRunner *backtestRunner) interrupt() {
	backtestRunner.progressMutex.Lock()
	backtestRunner.interrupted = true
	backtestRunner.progressMutex.Unlock()

	backtestRunner.stopReplaying()
}

func (backtestRunner *backtestRunner) wasInterrupted() bool {
	backtestRunner.progressMutex.Lock()
	defer backtestRunner.progressMutex.Unlock()

	return backtestRunner.interrupted
}

// end writes the ending and, for a success, the report in the same transaction; a refused ending means the run was cancelled or swept meanwhile and the report is dropped.
func (backtestRunner *backtestRunner) end(ended domains.BacktestRunDomain, result *entities.BacktestRunResult) {
	backtestRunner.progressMutex.Lock()
	defer backtestRunner.progressMutex.Unlock()

	backtestRunner.ended = true
	backtestRunner.backtestRun = ended
	service := backtestRunner.backtestRunService

	writeError := service.transactionRepository.Atomically(context.Background(),
		func(transactionContext context.Context) error {
			stillRunning, updateError := service.backtestRunRepository.UpdateIfRunning(
				transactionContext, ended.ToEntity())
			if updateError != nil || !stillRunning || result == nil {
				return updateError
			}

			return service.backtestRunResultRepository.Save(transactionContext, *result)
		})
	if writeError != nil {
		log.Printf("backtest run %d ending could not be written: %v", ended.ToEntity().ID, writeError)
	}
}
