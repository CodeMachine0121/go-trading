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

// LiveFollowRosterDutyCheckInterval is how often the job looks at the duty between refreshes, matching the duty's safety margin:
// a replica that leaves duty lets go of the places within it, before the next replica can take duty and the places.
const LiveFollowRosterDutyCheckInterval = 5 * time.Second

// LiveFollowRosterJob is separate from ingestion so a slow ingestion round cannot delay following a newly opened market.
// Only the replica on duty follows the roster, since a market caps how many symbols can be followed at once.
type LiveFollowRosterJob struct {
	kCandleFollowApplication *application.KCandleFollowApplication
	jobLeadershipApplication *application.JobLeadershipApplication
	interval                 time.Duration
	dutyCheckInterval        time.Duration
	// holdingRoster and lastRefreshedAt are touched only by the job's own goroutine.
	holdingRoster   bool
	lastRefreshedAt time.Time
	done            chan struct{}
	// finished closes when the job's goroutine has returned, in-flight round included.
	finished chan struct{}
	stopOnce func()
}

func NewLiveFollowRosterJob(
	kCandleFollowApplication *application.KCandleFollowApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
	dutyCheckInterval time.Duration,
) *LiveFollowRosterJob {
	done := make(chan struct{})

	return &LiveFollowRosterJob{
		kCandleFollowApplication: kCandleFollowApplication,
		jobLeadershipApplication: jobLeadershipApplication,
		interval:                 interval,
		dutyCheckInterval:        min(interval, dutyCheckInterval),
		done:                     done,
		finished:                 make(chan struct{}),
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
	defer close(liveFollowRosterJob.finished)

	liveFollowRosterJob.refresh(executionContext)

	ticker := time.NewTicker(liveFollowRosterJob.dutyCheckInterval)
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

// refresh, shared by the first pass and every tick, follows the roster as soon as this replica comes on duty and then every interval,
// and lets go of it as soon as the replica leaves duty, so the next replica on duty can take the places; it logs only failures.
func (liveFollowRosterJob *LiveFollowRosterJob) refresh(executionContext context.Context) {
	if !liveFollowRosterJob.jobLeadershipApplication.IsLeader() {
		if liveFollowRosterJob.holdingRoster {
			liveFollowRosterJob.kCandleFollowApplication.ReleaseFixedFollows()
			liveFollowRosterJob.holdingRoster = false
		}

		return
	}

	if liveFollowRosterJob.holdingRoster && time.Since(liveFollowRosterJob.lastRefreshedAt) < liveFollowRosterJob.interval {
		return
	}

	liveFollowRosterJob.holdingRoster = true
	liveFollowRosterJob.lastRefreshedAt = time.Now()
	if refreshError := liveFollowRosterJob.kCandleFollowApplication.
		RefreshFixedFollows(executionContext); refreshError != nil {
		log.Printf("live follow roster did not run: %v", refreshError)
	}
}

// Finished closes once the job has stopped and its in-flight round has ended.
func (liveFollowRosterJob *LiveFollowRosterJob) Finished() <-chan struct{} {
	return liveFollowRosterJob.finished
}
