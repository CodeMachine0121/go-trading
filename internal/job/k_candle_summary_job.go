package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// KCandleSummaryJob summarizes the watched symbols' recently finished days on start and every interval, only while this replica is on duty.
type KCandleSummaryJob struct {
	*repeatingRound
}

func NewKCandleSummaryJob(
	kCandleSummaryApplication *application.KCandleSummaryApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *KCandleSummaryJob {
	return &KCandleSummaryJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}

			if roundError := kCandleSummaryApplication.RunRound(executionContext); roundError != nil {
				log.Printf("k candle summary round did not finish: %v", roundError)
			}
		})}
}
