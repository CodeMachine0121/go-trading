package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// StrategyBotScanJob periodically runs due bots, reading state from the store each scan so restarts cost at most one interval instead of stopping bots.
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

// Start scans immediately so bots due during downtime run right away.
func (strategyBotScanJob *StrategyBotScanJob) Start(executionContext context.Context) {
	go strategyBotScanJob.run(executionContext)
}

// Stop lets in-flight rounds finish, since a half-done round could resend a message.
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

// scanOnce logs a failed scan and waits for the next one rather than killing the job.
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
