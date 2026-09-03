package job

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// KCandleIngestionInterval is how often the market is caught up with. It matches
// the length one K candle covers, which is a rule rather than a tuning knob: any
// other value would leave candles never fetched. Switching ingestion off is done by
// switching background jobs off or by watching nothing, not by changing this.
const KCandleIngestionInterval = 5 * time.Minute

// KCandleIngestionJob keeps the stored K candles current. It closes the gap left
// behind while nothing was running before it starts keeping up, and that ordering is
// the whole reason the two live in one job: it is expressed by the code running in
// sequence rather than by two jobs having to agree on who goes first.
type KCandleIngestionJob struct {
	kCandleIngestionApplication *application.KCandleIngestionApplication
	symbols                     []string
	interval                    time.Duration
	done                        chan struct{}
	stopOnce                    func()
}

// NewKCandleIngestionJob takes its own copy of the watchlist, so the list this job
// works from is settled here and cannot change underneath it afterwards.
func NewKCandleIngestionJob(
	kCandleIngestionApplication *application.KCandleIngestionApplication,
	symbols []string,
	interval time.Duration,
) *KCandleIngestionJob {
	done := make(chan struct{})

	return &KCandleIngestionJob{
		kCandleIngestionApplication: kCandleIngestionApplication,
		symbols:                     slices.Clone(symbols),
		interval:                    interval,
		done:                        done,
		stopOnce:                    sync.OnceFunc(func() { close(done) }),
	}
}

// Start hands the work to its own goroutine so that starting the system is not held
// up by a backfill that may have a lot of ground to make up. Every round it goes on
// to run is run under the context handed in here, so whoever started the job can
// still reach the work after it has been let go of.
func (kCandleIngestionJob *KCandleIngestionJob) Start(executionContext context.Context) {
	go kCandleIngestionJob.run(executionContext)
}

// Stop ends the job after the round it may be in the middle of. It asks for no round
// to be abandoned: a round halfway through storing candles is left to finish, which
// is why an orderly shutdown asks for this before it stops waiting.
func (kCandleIngestionJob *KCandleIngestionJob) Stop() {
	kCandleIngestionJob.stopOnce()
}

// run backfills first and only then begins keeping up, which is the ordering the
// two halves have to be in: a round that overlapped the backfill would have both
// halves writing the same candle.
func (kCandleIngestionJob *KCandleIngestionJob) run(executionContext context.Context) {
	backfillReport, backfillError := kCandleIngestionJob.kCandleIngestionApplication.
		RunBackfill(executionContext, kCandleIngestionJob.symbols)
	kCandleIngestionJob.report("startup backfill", backfillReport, backfillError)

	ticker := time.NewTicker(kCandleIngestionJob.interval)
	defer ticker.Stop()

	for {
		// Being stopped and the context being done are both reasons to end, and the
		// difference between them is what already happened rather than what happens
		// here: a stop let the round in hand finish, a done context did not.
		select {
		case <-kCandleIngestionJob.done:
			return
		case <-executionContext.Done():
			return
		case <-ticker.C:
			// A select picks at random among the cases that are ready, and a tick
			// can already be waiting in the channel — which is what happens
			// whenever a round outruns the interval, a market source that will not
			// answer being the ordinary way. Without this second look, a job told
			// to stop at that moment starts one more round about half the time,
			// and "take on no further rounds" would be a coin toss.
			select {
			case <-kCandleIngestionJob.done:
				return
			case <-executionContext.Done():
				return
			default:
			}

			roundReport, roundError := kCandleIngestionJob.kCandleIngestionApplication.
				RunScheduledRound(executionContext, kCandleIngestionJob.symbols)
			kCandleIngestionJob.report("scheduled round", roundReport, roundError)
		}
	}
}

// report writes down only what went wrong, and in enough detail to act on: which
// trading symbol, which candle, and which rule it broke. A round with nothing to
// say stays quiet.
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
