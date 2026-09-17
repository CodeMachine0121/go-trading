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

// ErrKCandleHistorySyncRunNotFound marks an identifier no history sync was ever
// recorded under.
var ErrKCandleHistorySyncRunNotFound = errors.New("k candle history sync run not found")

// kCandleHistorySyncInterrupted is what a run cut off by a restart is left saying. It
// reads as an instruction rather than a diagnosis, because the one thing the person
// looking at it can do is ask for the stretch again — and asking again is cheap, since
// everything already stored is never fetched twice.
const kCandleHistorySyncInterrupted = "interrupted by restart"

// progressWriteAttempts and endingWriteAttempts are how hard each kind of write tries.
//
// A progress figure is worth one go: the next chunk writes it again a moment later,
// and a run that stopped to retry a number nobody is reading yet is a run spending its
// source allowance on nothing. The closing write is the last thing that will ever be
// said about this run, so it is worth waiting on.
const (
	progressWriteAttempts = 1
	endingWriteAttempts   = 3
	betweenWriteAttempts  = 2 * time.Second
)

// kCandleHistorySyncRunner is one history sync being fetched: which run it is filling
// in, whose history, and the chunks still to walk.
//
// It is not a domain model and does not live with them. What it holds is execution —
// a place in storage to write progress to, and a walk through a source in progress —
// and taking those away leaves no domain concept behind. Every rule it obeys belongs
// to the ingestion service and the domain models under it; this only drives them and
// records where they got to.
//
// It exists at all because of one requirement: the fetch must outlive the request that
// asked for it. Four years of candles is tens of minutes of paced requests, and no
// connection stays open that long — so the work is driven from here and the caller is
// handed a run to come back and look at.
type kCandleHistorySyncRunner struct {
	kCandleIngestionService *KCandleIngestionService
	// syncRun was written before any of this started, which is what makes the work
	// findable while it is still going.
	syncRun          entities.KCandleHistorySyncRun
	registeredSymbol entities.TradingSymbol
	ingestionDomain  domains.KCandleIngestionDomain
	chunks           []vo.KCandleFetchWindowVo
	// completedChunks and symbolReport are how far the walk actually got, kept here
	// rather than read off the return so that an ending — including one nobody
	// planned, like a panic — reports what really happened instead of nothing.
	completedChunks int
	symbolReport    dto.KCandleSymbolIngestionReportDto
}

// run walks the stretch to an end and records it, either way.
//
// **It takes no context from the caller and this is the whole point.** Given the
// request's context, the fetch would be cancelled the instant the caller's connection
// went away — which is exactly the case this was built for. What bounds it instead is
// each request's own timeout, and what cleans up after a shutdown is the sweep at
// startup.
//
// Neither ending returns anything. There is nobody left to return to: the request was
// answered with a place to look, and this is what fills that place in.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) run() {
	executionContext := context.Background()

	// A panic out here has nothing above it to contain it, so it would stop the API,
	// the background jobs and every other fetch in flight. The run is closed as failed
	// on the way out so the row does not sit at running until the next restart sweeps
	// it.
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

// recordProgress brings the run up to date after each chunk, so that somebody looking
// sees a number that moves rather than one that only appears at the end.
//
// A write per chunk is deliberate. It is one small statement against thousands of
// paced requests, and a progress figure that arrives in batches is a figure nobody can
// tell from a run that has stalled.
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

// recordEnding closes the run at wherever the walk actually reached.
//
// **The chunk count is the one it got to, not the one it was given.** A run that gave
// up at chunk five of fifteen hundred reporting 1500 of 1500 would be worse than no
// figure at all: it reads as finished, and the pair exists precisely so that a long
// run can be told apart from a stalled one.
//
// An empty failure reason is a run that walked the whole stretch. The source having
// refused is carried separately, because a source refusing is something the run found
// out rather than something the run did wrong.
func (kCandleHistorySyncRunner *kCandleHistorySyncRunner) recordEnding(
	executionContext context.Context, failureReason string,
) {
	// Read now, not when the run was accepted. The moment it started is already on the
	// row; taking the ending from the same reading would make every run, however long,
	// look instantaneous.
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

// save writes the run, trying again as many times as the caller thinks the write is
// worth, and giving up loudly rather than silently.
//
// **A progress write and the closing write are not worth the same.** Losing a progress
// figure costs one stale number until the next chunk lands; losing the closing write
// leaves the row saying running for as long as this process lives — nothing retries
// it, and the sweep that would catch it only happens at start-up. So somebody polling
// that run waits for an ending that has already happened and will never be written.
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
