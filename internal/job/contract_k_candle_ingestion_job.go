package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ContractKCandleIngestionJob keeps the stored perpetual contract K candles current.
// It closes the gap left behind while nothing was running before it starts keeping
// up, and that ordering is the whole reason the two live in one job: it is expressed
// by the code running in sequence rather than by two jobs having to agree on who goes
// first.
//
// It is a job of its own rather than more work inside the spot one. The two fetch
// from different venues on different allowances, and one refusing to answer must not
// hold the other up — which is exactly what sharing a round would do.
type ContractKCandleIngestionJob struct {
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication
	interval                            time.Duration
	done                                chan struct{}
	stopOnce                            func()
}

// NewContractKCandleIngestionJob knows nothing about which contracts are watched.
// That list belongs to the system rather than to this job, and each round reads it
// afresh — so changing it is a change to the system, not a reason to restart it.
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

// Start hands the work to its own goroutine so that starting the system is not held
// up by a backfill that may have a lot of ground to make up.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) Start(
	executionContext context.Context,
) {
	go contractKCandleIngestionJob.run(executionContext)
}

// Stop ends the job after the round it may be in the middle of. It asks for no round
// to be abandoned: a round halfway through storing candles is left to finish.
func (contractKCandleIngestionJob *ContractKCandleIngestionJob) Stop() {
	contractKCandleIngestionJob.stopOnce()
}

// run backfills first and only then begins keeping up, which is the ordering the two
// halves have to be in: a round that overlapped the backfill would have both halves
// writing the same candle.
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
			// A select picks at random among the cases that are ready, and a tick can
			// already be waiting in the channel — which is what happens whenever a
			// round outruns the interval. Without this second look, a job told to stop
			// at that moment starts one more round about half the time.
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

// report writes down only what went wrong, and in enough detail to act on: which
// contract, which candle, and which rule it broke. A round with nothing to say stays
// quiet.
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
