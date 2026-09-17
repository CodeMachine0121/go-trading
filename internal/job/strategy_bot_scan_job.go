package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// StrategyBotScanJob wakes up on a timer and runs whichever bots have come due.
//
// One job that scans, rather than one goroutine per bot. A goroutine per bot would
// hold "this bot is running" in a process, and a process restarts: every bot would
// stop, silently, with its owner still believing it was watching. Here the question
// is asked of the store every time, so a restart costs at most one scan interval and
// nobody has to press play again.
//
// It sequences nothing and knows nothing about strategy scripts, scripts, conditions or
// Telegram. Whether a round halts a bot or waits for the next one is decided far
// behind this, which is why nothing here can decide it differently.
type StrategyBotScanJob struct {
	strategyBotRunApplication *application.StrategyBotRunApplication
	interval                  time.Duration
	done                      chan struct{}
	stopOnce                  func()
}

func NewStrategyBotScanJob(
	strategyBotRunApplication *application.StrategyBotRunApplication,
	interval time.Duration,
) *StrategyBotScanJob {
	done := make(chan struct{})

	return &StrategyBotScanJob{
		strategyBotRunApplication: strategyBotRunApplication,
		interval:                  interval,
		done:                      done,
		stopOnce:                  sync.OnceFunc(func() { close(done) }),
	}
}

// Start scans once straight away and then keeps scanning.
//
// The first scan is immediate because a system that has just come up is running
// nothing, and every bot that was due while it was down is due now. Waiting a whole
// interval to notice would add that wait to an outage somebody is already living
// with.
func (strategyBotScanJob *StrategyBotScanJob) Start(executionContext context.Context) {
	go strategyBotScanJob.run(executionContext)
}

// Stop ends the job after the scan it may be in the middle of. Rounds already under
// way finish: a round that stopped halfway would have sent a message and never
// recorded that it did, and the next scan would send it again.
func (strategyBotScanJob *StrategyBotScanJob) Stop() {
	strategyBotScanJob.stopOnce()
}

func (strategyBotScanJob *StrategyBotScanJob) run(executionContext context.Context) {
	ticker := time.NewTicker(strategyBotScanJob.interval)
	defer ticker.Stop()

	strategyBotScanJob.scanOnce(executionContext)

	for {
		select {
		case <-executionContext.Done():
			return
		case <-strategyBotScanJob.done:
			return
		case <-ticker.C:
			strategyBotScanJob.scanOnce(executionContext)
		}
	}
}

// scanOnce runs the bots that are due.
//
// A scan that fails is logged and left: the next one is a minute away, and there is
// nothing here that could do better with the failure than try again. Killing the job
// would mean one unreachable database switching off every bot in the system.
func (strategyBotScanJob *StrategyBotScanJob) scanOnce(executionContext context.Context) {
	roundsRun, scanError := strategyBotScanJob.strategyBotRunApplication.RunDueRounds(executionContext)
	if scanError != nil {
		log.Printf("strategy bot scan failed: %v", scanError)
		return
	}

	if roundsRun > 0 {
		log.Printf("strategy bot scan ran %d round(s)", roundsRun)
	}
}
