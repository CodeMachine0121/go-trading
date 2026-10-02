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
// Like the spot job it works only on duty and backfills again on every return to duty.
type ContractKCandleIngestionJob struct {
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication
	jobLeadershipApplication            *application.JobLeadershipApplication
	interval                            time.Duration
	done                                chan struct{}
	stopOnce                            func()
	// needsBackfill is touched only by the job's own goroutine.
	needsBackfill bool
}

// NewContractKCandleIngestionJob reads the watched contracts afresh each round.
func NewContractKCandleIngestionJob(
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *ContractKCandleIngestionJob {
	done := make(chan struct{})

	return &ContractKCandleIngestionJob{
		kCandleContractIngestionApplication: kCandleContractIngestionApplication,
		jobLeadershipApplication:            jobLeadershipApplication,
		interval:                            interval,
		done:                                done,
		stopOnce:                            sync.OnceFunc(func() { close(done) }),
		needsBackfill:                       true,
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

// run finishes each backfill before the next round so the two never write the same candle.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) run(
	executionContext context.Context,
) {
	contractKCandleIngestionJob.runRound(executionContext)

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

			contractKCandleIngestionJob.runRound(executionContext)
		}
	}
}

// runRound, shared by the first round and every tick, backfills on the first round on duty and keeps up afterwards; off duty it only remembers to backfill.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) runRound(executionContext context.Context) {
	if !contractKCandleIngestionJob.jobLeadershipApplication.IsLeader() {
		contractKCandleIngestionJob.needsBackfill = true

		return
	}

	if contractKCandleIngestionJob.needsBackfill {
		backfillReport, backfillError := contractKCandleIngestionJob.
			kCandleContractIngestionApplication.RunBackfill(executionContext)
		contractKCandleIngestionJob.report("backfill", backfillReport, backfillError)
		// Kept when the backfill could not run, so the next round tries again instead of leaving the gap.
		contractKCandleIngestionJob.needsBackfill = backfillError != nil

		return
	}

	roundReport, roundError := contractKCandleIngestionJob.
		kCandleContractIngestionApplication.RunScheduledRound(executionContext)
	contractKCandleIngestionJob.report("scheduled round", roundReport, roundError)
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
