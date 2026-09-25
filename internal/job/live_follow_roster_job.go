package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// LiveFollowRosterInterval matches the K candle length so a newly opened market waits at most one candle to be followed.
const LiveFollowRosterInterval = 5 * time.Minute

// LiveFollowRosterJob is separate from ingestion so a slow ingestion round cannot delay following a newly opened market.
type LiveFollowRosterJob struct {
	kCandleFollowApplication *application.KCandleFollowApplication
	interval                 time.Duration
	done                     chan struct{}
	stopOnce                 func()
}

func NewLiveFollowRosterJob(
	kCandleFollowApplication *application.KCandleFollowApplication,
	interval time.Duration,
) *LiveFollowRosterJob {
	done := make(chan struct{})

	return &LiveFollowRosterJob{
		kCandleFollowApplication: kCandleFollowApplication,
		interval:                 interval,
		done:                     done,
		stopOnce:                 sync.OnceFunc(func() { close(done) }),
	}
}

// Start runs the first pass immediately so a restart during trading hours follows markets right away.
func (liveFollowRosterJob *LiveFollowRosterJob) Start(executionContext context.Context) {
	go liveFollowRosterJob.run(executionContext)
}

// Stop lets an in-flight pass finish.
func (liveFollowRosterJob *LiveFollowRosterJob) Stop() {
	liveFollowRosterJob.stopOnce()
}

func (liveFollowRosterJob *LiveFollowRosterJob) run(executionContext context.Context) {
	liveFollowRosterJob.refresh(executionContext)

	ticker := time.NewTicker(liveFollowRosterJob.interval)
	defer ticker.Stop()

	for {
		select {
		case <-liveFollowRosterJob.done:
			return
		case <-executionContext.Done():
			return
		case <-ticker.C:
			// Re-check stop because select picks randomly when a tick is already pending.
			select {
			case <-liveFollowRosterJob.done:
				return
			case <-executionContext.Done():
				return
			default:
			}

			liveFollowRosterJob.refresh(executionContext)
		}
	}
}

// refresh logs only failures.
func (liveFollowRosterJob *LiveFollowRosterJob) refresh(executionContext context.Context) {
	if refreshError := liveFollowRosterJob.kCandleFollowApplication.
		RefreshFixedFollows(executionContext); refreshError != nil {
		log.Printf("live follow roster did not run: %v", refreshError)
	}
}
