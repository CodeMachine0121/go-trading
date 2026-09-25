package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// KCandleIngestionInterval must equal the K candle length or candles would be skipped; it is not a tuning knob.
const KCandleIngestionInterval = domains.KCandleInterval

// KCandleIngestionJob backfills before keeping up, in one job so the ordering is sequential code.
type KCandleIngestionJob struct {
	kCandleIngestionApplication *application.KCandleIngestionApplication
	interval                    time.Duration
	done                        chan struct{}
	stopOnce                    func()
}

// NewKCandleIngestionJob reads the watched markets afresh each round.
func NewKCandleIngestionJob(
	kCandleIngestionApplication *application.KCandleIngestionApplication,
	interval time.Duration,
) *KCandleIngestionJob {
	done := make(chan struct{})

	return &KCandleIngestionJob{
		kCandleIngestionApplication: kCandleIngestionApplication,
		interval:                    interval,
		done:                        done,
		stopOnce:                    sync.OnceFunc(func() { close(done) }),
	}
}

// Start runs rounds on its own goroutine under the given context.
func (kCandleIngestionJob *KCandleIngestionJob) Start(executionContext context.Context) {
	go kCandleIngestionJob.run(executionContext)
}

// Stop lets an in-flight round finish.
func (kCandleIngestionJob *KCandleIngestionJob) Stop() {
	kCandleIngestionJob.stopOnce()
}

// run finishes the backfill before starting rounds so the two never write the same candle.
func (kCandleIngestionJob *KCandleIngestionJob) run(executionContext context.Context) {
	backfillReport, backfillError := kCandleIngestionJob.kCandleIngestionApplication.
		RunBackfill(executionContext)
	kCandleIngestionJob.report("startup backfill", backfillReport, backfillError)

	ticker := time.NewTicker(kCandleIngestionJob.interval)
	defer ticker.Stop()

	for {
		// A stop lets the round in hand finish; a done context does not.
		select {
		case <-kCandleIngestionJob.done:
			return
		case <-executionContext.Done():
			return
		case <-ticker.C:
			// Re-check stop because select picks randomly when a tick is already pending.
			select {
			case <-kCandleIngestionJob.done:
				return
			case <-executionContext.Done():
				return
			default:
			}

			roundReport, roundError := kCandleIngestionJob.kCandleIngestionApplication.
				RunScheduledRound(executionContext)
			kCandleIngestionJob.report("scheduled round", roundReport, roundError)
		}
	}
}

// report logs only failures.
func (kCandleIngestionJob *KCandleIngestionJob) report(
	stage string,
	ingestionReport dto.KCandleIngestionReportDto,
	runError error,
) {
	if runError != nil {
		log.Printf("k candle %s did not run: %v", stage, runError)
		return
	}

	for _, symbolReport := range ingestionReport.SymbolReports {
		if symbolReport.FetchFailureReason != "" {
			log.Printf("k candle %s got no answer for %s: %s",
				stage, symbolReport.Symbol, symbolReport.FetchFailureReason)
		}

		for _, skippedKCandle := range symbolReport.SkippedKCandles {
			log.Printf("k candle %s skipped %s at %s: %s",
				stage, symbolReport.Symbol,
				skippedKCandle.OpenTime.Format(time.RFC3339), skippedKCandle.Reason)
		}
	}
}
