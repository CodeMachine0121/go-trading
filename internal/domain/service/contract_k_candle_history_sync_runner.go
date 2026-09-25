package service

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// contractKCandleHistorySyncRunner drives one contract history sync off the request so the fetch outlives it; all rules belong to the ingestion service.
type contractKCandleHistorySyncRunner struct {
	contractKCandleIngestionService *ContractKCandleIngestionService
	syncRun                         entities.KCandleContractHistorySyncRun
	registeredSymbol                entities.ContractTradingSymbol
	ingestionDomain                 domains.KCandleIngestionDomain
	chunks                          []vo.KCandleFetchWindowVo
	// completedChunks and symbolReport track actual progress so even an unplanned ending (e.g. a panic) reports it.
	completedChunks int
	symbolReport    dto.KCandleSymbolIngestionReportDto
	// positionStatisticHistory and positionStatisticProgress track the statistics walk the same way.
	positionStatisticHistory  domains.ContractPositionStatisticHistoryDomain
	positionStatisticProgress dto.ContractPositionStatisticSyncProgressDto
}

// run deliberately ignores the caller's context so a dropped connection cannot cancel the fetch.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) run() {
	executionContext := context.Background()

	// Off the request a panic would crash the process; recover and close the run as failed.
	defer func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}

		log.Printf("contract k candle history sync %d panicked: %v\n%s",
			contractKCandleHistorySyncRunner.syncRun.ID, panicValue, debug.Stack())

		contractKCandleHistorySyncRunner.recordEnding(
			executionContext, "contract k candle history sync broke down")
	}()

	syncError := contractKCandleHistorySyncRunner.contractKCandleIngestionService.syncSymbolHistory(
		executionContext,
		contractKCandleHistorySyncRunner.registeredSymbol,
		contractKCandleHistorySyncRunner.ingestionDomain,
		contractKCandleHistorySyncRunner.chunks,
		contractKCandleHistorySyncRunner.recordProgress,
	)
	if syncError != nil {
		contractKCandleHistorySyncRunner.recordEnding(executionContext, syncError.Error())

		return
	}

	// The candle source refusing does not skip the statistics, which come from a different source.
	statisticError := contractKCandleHistorySyncRunner.contractKCandleIngestionService.
		positionStatisticService.syncHistory(
		executionContext,
		contractKCandleHistorySyncRunner.registeredSymbol.Symbol,
		contractKCandleHistorySyncRunner.positionStatisticHistory,
		contractKCandleHistorySyncRunner.recordPositionStatisticProgress,
	)
	if statisticError != nil {
		contractKCandleHistorySyncRunner.recordEnding(executionContext, statisticError.Error())

		return
	}

	contractKCandleHistorySyncRunner.recordEnding(executionContext, "")
}

// recordProgress updates the run after each chunk so progress is visible.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordProgress(
	completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto,
) {
	contractKCandleHistorySyncRunner.completedChunks = completedChunks
	contractKCandleHistorySyncRunner.symbolReport = symbolReport

	contractKCandleHistorySyncRunner.save(
		context.Background(), contractKCandleHistorySyncRunner.currentRun(), progressWriteAttempts)
}

func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordPositionStatisticProgress(
	progress dto.ContractPositionStatisticSyncProgressDto,
) {
	contractKCandleHistorySyncRunner.positionStatisticProgress = progress

	contractKCandleHistorySyncRunner.save(
		context.Background(), contractKCandleHistorySyncRunner.currentRun(), progressWriteAttempts)
}

// currentRun merges both walks' progress so neither write rolls back the other.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) currentRun() entities.KCandleContractHistorySyncRun {
	syncRun := contractKCandleHistorySyncRunner.syncRun
	syncRun.CompletedChunks = contractKCandleHistorySyncRunner.completedChunks
	syncRun.StoredCount = contractKCandleHistorySyncRunner.symbolReport.StoredCount
	syncRun.SkippedCount = contractKCandleHistorySyncRunner.symbolReport.SkippedCount
	syncRun.FetchFailureReason = contractKCandleHistorySyncRunner.symbolReport.FetchFailureReason

	statisticProgress := contractKCandleHistorySyncRunner.positionStatisticProgress
	syncRun.PositionStatisticCompletedDays = statisticProgress.CompletedDays
	syncRun.PositionStatisticStoredCount = statisticProgress.StoredCount
	syncRun.PositionStatisticSkippedCount = statisticProgress.SkippedCount
	syncRun.PositionStatisticFetchFailureReason = statisticProgress.FetchFailureReason

	return syncRun
}

// recordEnding closes the run with the counts actually reached (never the planned totals); a source refusal is recorded separately from a failure reason.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordEnding(
	executionContext context.Context, failureReason string,
) {
	// Read the clock now so the run's duration is real.
	finishedAt := contractKCandleHistorySyncRunner.contractKCandleIngestionService.clockProxy.Now()

	syncRun := contractKCandleHistorySyncRunner.currentRun()
	syncRun.FailureReason = failureReason
	syncRun.FinishedAt = &finishedAt
	syncRun.Status = string(vo.KCandleHistorySyncSucceeded)
	if failureReason != "" {
		syncRun.Status = string(vo.KCandleHistorySyncFailed)
	}

	contractKCandleHistorySyncRunner.save(executionContext, syncRun, endingWriteAttempts)
}

// save retries the write the given number of times and fails loudly.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) save(
	executionContext context.Context, syncRun entities.KCandleContractHistorySyncRun, attempts int,
) {
	for attempt := range attempts {
		_, saveError := contractKCandleHistorySyncRunner.contractKCandleIngestionService.
			kCandleContractHistorySyncRunRepository.Save(executionContext, syncRun)
		if saveError == nil {
			return
		}

		log.Printf("contract k candle history sync %d could not be brought up to date: %v",
			syncRun.ID, saveError)

		if attempt+1 < attempts {
			contractKCandleHistorySyncRunner.contractKCandleIngestionService.clockProxy.Sleep(
				betweenWriteAttempts)
		}
	}
}
