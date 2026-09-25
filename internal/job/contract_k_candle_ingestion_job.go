package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ContractKCandleIngestionJob backfills before keeping up, and is separate from the spot job so one venue's failures cannot stall the other.
type ContractKCandleIngestionJob struct {
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication
	interval                            time.Duration
	done                                chan struct{}
	stopOnce                            func()
}

// NewContractKCandleIngestionJob reads the watched contracts afresh each round.
func NewContractKCandleIngestionJob(
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication,
	interval time.Duration,
) *ContractKCandleIngestionJob {
	done := make(chan struct{})

	return &ContractKCandleIngestionJob{
		kCandleContractIngestionApplication: kCandleContractIngestionApplication,
		interval:                            interval,
		done:                                done,
		stopOnce:                            sync.OnceFunc(func() { close(done) }),
	}
}

func (contractKCandleIngestionJob *ContractKCandleIngestionJob) Start(
	executionContext context.Context,
) {
	go contractKCandleIngestionJob.run(executionContext)
}

// Stop lets an in-flight round finish.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) Stop() {
	contractKCandleIngestionJob.stopOnce()
}

// run finishes the backfill before starting rounds so the two never write the same candle.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) run(
	executionContext context.Context,
) {
	backfillReport, backfillError := contractKCandleIngestionJob.
		kCandleContractIngestionApplication.RunBackfill(executionContext)
	contractKCandleIngestionJob.report("startup backfill", backfillReport, backfillError)

	ticker := time.NewTicker(contractKCandleIngestionJob.interval)
	defer ticker.Stop()

	for {
		select {
		case <-contractKCandleIngestionJob.done:
			return
		case <-executionContext.Done():
			return
		case <-ticker.C:
			// Re-check stop because select picks randomly when a tick is already pending.
			select {
			case <-contractKCandleIngestionJob.done:
				return
			case <-executionContext.Done():
				return
			default:
			}

			roundReport, roundError := contractKCandleIngestionJob.
				kCandleContractIngestionApplication.RunScheduledRound(executionContext)
			contractKCandleIngestionJob.report("scheduled round", roundReport, roundError)
		}
	}
}

// report logs only failures.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) report(
	stage string,
	ingestionReport dto.KCandleIngestionReportDto,
	runError error,
) {
	if runError != nil {
		log.Printf("contract k candle %s did not run: %v", stage, runError)

		return
	}

	for _, symbolReport := range ingestionReport.SymbolReports {
		if symbolReport.FetchFailureReason != "" {
			log.Printf("contract k candle %s got no answer for %s: %s",
				stage, symbolReport.Symbol, symbolReport.FetchFailureReason)
		}

		for _, skippedKCandle := range symbolReport.SkippedKCandles {
			log.Printf("contract k candle %s skipped %s at %s: %s",
				stage, symbolReport.Symbol,
				skippedKCandle.OpenTime.Format(time.RFC3339), skippedKCandle.Reason)
		}
	}
}
