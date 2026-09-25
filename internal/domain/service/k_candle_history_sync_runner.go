package service

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

var ErrKCandleHistorySyncRunNotFound = errors.New("k candle history sync run not found")

// kCandleHistorySyncInterrupted is the failure reason for a run cut off by a restart; re-requesting is cheap because stored candles are never fetched twice.
const kCandleHistorySyncInterrupted = "interrupted by restart"

// A progress write gets one attempt since the next chunk rewrites it; the closing write is the run's last word, so it is retried.
const (
	progressWriteAttempts = 1
	endingWriteAttempts   = 3
	betweenWriteAttempts  = 2 * time.Second
)

// kCandleHistorySyncRunner drives one history sync in the background, because a multi-year fetch outlives any request; it holds execution state only, the rules live in the ingestion service.
type kCandleHistorySyncRunner struct {
	kCandleIngestionService *KCandleIngestionService
	// syncRun is persisted before the walk starts so the run is findable while in progress.
	syncRun          entities.KCandleHistorySyncRun
	registeredSymbol entities.TradingSymbol
	ingestionDomain  domains.KCandleIngestionDomain
	chunks           []vo.KCandleFetchWindowVo
	// completedChunks and symbolReport track real progress so any ending, even a panic, reports what actually happened.
	completedChunks int
	symbolReport    dto.KCandleSymbolIngestionReportDto
}

// run walks the stretch and records the ending; it deliberately uses a background context so the fetch survives the caller disconnecting.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) run() {
	executionContext := context.Background()

	// An unrecovered panic here would crash the whole process; recover and close the run as failed so it does not stay running until restart.
	defer func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}

		log.Printf("k candle history sync %d panicked: %v\n%s",
			kCandleHistorySyncRunner.syncRun.ID, panicValue, debug.Stack())

		kCandleHistorySyncRunner.recordEnding(executionContext, "k candle history sync broke down")
	}()

	syncError := kCandleHistorySyncRunner.kCandleIngestionService.syncSymbolHistory(
		executionContext,
		kCandleHistorySyncRunner.registeredSymbol,
		kCandleHistorySyncRunner.ingestionDomain,
		kCandleHistorySyncRunner.chunks,
		kCandleHistorySyncRunner.recordProgress,
	)
	if syncError != nil {
		kCandleHistorySyncRunner.recordEnding(executionContext, syncError.Error())

		return
	}

	kCandleHistorySyncRunner.recordEnding(executionContext, "")
}

// recordProgress persists progress after every chunk so a slow run can be told from a stalled one.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) recordProgress(
	completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto,
) {
	kCandleHistorySyncRunner.completedChunks = completedChunks
	kCandleHistorySyncRunner.symbolReport = symbolReport

	syncRun := kCandleHistorySyncRunner.syncRun
	syncRun.CompletedChunks = completedChunks
	syncRun.StoredCount = symbolReport.StoredCount
	syncRun.SkippedCount = symbolReport.SkippedCount

	kCandleHistorySyncRunner.save(context.Background(), syncRun, progressWriteAttempts)
}

// recordEnding closes the run at the chunk actually reached, not the planned total, so an early failure never reads as finished.
// An empty failure reason means success; a source refusal is recorded separately in FetchFailureReason.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) recordEnding(
	executionContext context.Context, failureReason string,
) {
	// Read now so the recorded duration is real.
	finishedAt := kCandleHistorySyncRunner.kCandleIngestionService.clockProxy.Now()

	syncRun := kCandleHistorySyncRunner.syncRun
	syncRun.CompletedChunks = kCandleHistorySyncRunner.completedChunks
	syncRun.StoredCount = kCandleHistorySyncRunner.symbolReport.StoredCount
	syncRun.SkippedCount = kCandleHistorySyncRunner.symbolReport.SkippedCount
	syncRun.FetchFailureReason = kCandleHistorySyncRunner.symbolReport.FetchFailureReason
	syncRun.FailureReason = failureReason
	syncRun.FinishedAt = &finishedAt
	syncRun.Status = string(vo.KCandleHistorySyncSucceeded)
	if failureReason != "" {
		syncRun.Status = string(vo.KCandleHistorySyncFailed)
	}

	kCandleHistorySyncRunner.save(executionContext, syncRun, endingWriteAttempts)
}

// save writes the run up to attempts times, logging each failure; a lost closing write would leave the row running until the next restart sweep.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) save(
	executionContext context.Context, syncRun entities.KCandleHistorySyncRun, attempts int,
) {
	for attempt := range attempts {
		_, saveError := kCandleHistorySyncRunner.kCandleIngestionService.
			kCandleHistorySyncRunRepository.Save(executionContext, syncRun)
		if saveError == nil {
			return
		}

		log.Printf("k candle history sync %d could not be brought up to date: %v",
			syncRun.ID, saveError)

		if attempt+1 < attempts {
			kCandleHistorySyncRunner.kCandleIngestionService.clockProxy.Sleep(
				betweenWriteAttempts)
		}
	}
}
