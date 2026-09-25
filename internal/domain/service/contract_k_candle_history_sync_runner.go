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

// contractKCandleHistorySyncRunner is one contract history sync being fetched: which
// run it is filling in, whose history, and the chunks still to walk.
//
// It is not a domain model and does not live with them. What it holds is execution,
// and taking that away leaves no domain concept behind. Every rule it obeys belongs to
// the ingestion service and the domain models under it; this only drives them and
// records where they got to.
//
// It exists for one requirement: the fetch must outlive the request that asked for it.
type contractKCandleHistorySyncRunner struct {
	contractKCandleIngestionService *ContractKCandleIngestionService
	// syncRun was written before any of this started, which is what makes the work
	// findable while it is still going.
	syncRun          entities.KCandleContractHistorySyncRun
	registeredSymbol entities.ContractTradingSymbol
	ingestionDomain  domains.KCandleIngestionDomain
	chunks           []vo.KCandleFetchWindowVo
	// completedChunks and symbolReport are how far the walk actually got, kept here
	// rather than read off the return so that an ending — including one nobody
	// planned, like a panic — reports what really happened instead of nothing.
	completedChunks int
	symbolReport    dto.KCandleSymbolIngestionReportDto
	// positionStatisticHistory is the stretch of position statistics walked once the
	// candles are done, and positionStatisticProgress how far that walk got — kept the
	// same way, and for the same reason, as the two above.
	positionStatisticHistory  domains.ContractPositionStatisticHistoryDomain
	positionStatisticProgress dto.ContractPositionStatisticSyncProgressDto
}

// run walks the stretch to an end and records it, either way.
//
// **It takes no context from the caller and this is the whole point.** Given the
// request's context, the fetch would be cancelled the instant the caller's connection
// went away — which is exactly the case this was built for.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) run() {
	executionContext := context.Background()

	// A panic out here has nothing above it to contain it, so it would stop the API,
	// the background jobs and every other fetch in flight. The run is closed as failed
	// on the way out so the row does not sit at running until the next restart.
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
		// This system broke on the candles, so it is not trusted with the statistics.
		contractKCandleHistorySyncRunner.recordEnding(executionContext, syncError.Error())

		return
	}

	// The candle source refusing is not a reason to skip the statistics: they come
	// from a different source, and that one may well answer.
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

// recordProgress brings the run up to date after each chunk, so that somebody looking
// sees a number that moves rather than one that only appears at the end.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordProgress(
	completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto,
) {
	contractKCandleHistorySyncRunner.completedChunks = completedChunks
	contractKCandleHistorySyncRunner.symbolReport = symbolReport

	contractKCandleHistorySyncRunner.save(
		context.Background(), contractKCandleHistorySyncRunner.currentRun(), progressWriteAttempts)
}

// recordPositionStatisticProgress brings the run up to date after each day of position
// statistics, for the reason recordProgress does after each chunk of candles.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordPositionStatisticProgress(
	progress dto.ContractPositionStatisticSyncProgressDto,
) {
	contractKCandleHistorySyncRunner.positionStatisticProgress = progress

	contractKCandleHistorySyncRunner.save(
		context.Background(), contractKCandleHistorySyncRunner.currentRun(), progressWriteAttempts)
}

// currentRun is the run as far as both walks have actually got. Every write goes
// through it, so a progress write about the candles never rolls back what the
// statistics had reached, nor the other way round.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) currentRun() entities.KCandleContractHistorySyncRun {
	syncRun := contractKCandleHistorySyncRunner.syncRun
	syncRun.CompletedChunks = contractKCandleHistorySyncRunner.completedChunks
	syncRun.StoredCount = contractKCandleHistorySyncRunner.symbolReport.StoredCount
	syncRun.SkippedCount = contractKCandleHistorySyncRunner.symbolReport.SkippedCount
	syncRun.FetchFailureReason = contractKCandleHistorySyncRunner.symbolReport.FetchFailureReason

	// The number of days was written when the run was, and never changes.
	statisticProgress := contractKCandleHistorySyncRunner.positionStatisticProgress
	syncRun.PositionStatisticCompletedDays = statisticProgress.CompletedDays
	syncRun.PositionStatisticStoredCount = statisticProgress.StoredCount
	syncRun.PositionStatisticSkippedCount = statisticProgress.SkippedCount
	syncRun.PositionStatisticFetchFailureReason = statisticProgress.FetchFailureReason

	return syncRun
}

// recordEnding closes the run at wherever both walks actually reached.
//
// **The chunk and day counts are the ones it got to, not the ones it was given.** A run that gave
// up at chunk five of fifteen hundred reporting 1500 of 1500 would be worse than no
// figure at all: it reads as finished.
//
// An empty failure reason is a run that walked the whole stretch. The source having
// refused is carried separately, because a source refusing is something the run found
// out rather than something the run did wrong — which is also what a contract that did
// not exist over the stretch looks like.
func (contractKCandleHistorySyncRunner *contractKCandleHistorySyncRunner) recordEnding(
	executionContext context.Context, failureReason string,
) {
	// Read now, not when the run was accepted. Taking the ending from the same reading
	// would make every run, however long, look instantaneous.
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

// save writes the run, trying again as many times as the caller thinks the write is
// worth, and giving up loudly rather than silently.
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
