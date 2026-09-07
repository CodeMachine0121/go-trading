package job

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// LiveFollowRosterInterval is how often each limited market's live places are handed
// out again. It matches the length one K candle covers, so the longest anybody waits
// for a market to be followed after it opens — or after the watchlist changed — is
// the same wait as for a candle. Making it shorter would buy a minute at the price of
// a second number that has to stay in step with this one.
const LiveFollowRosterInterval = 5 * time.Minute

// LiveFollowRosterJob keeps the live places of every market that limits them handed
// out to the right symbols.
//
// It is its own job rather than another step of the ingestion round because the two
// answer to different things. A round that is slow — a source not answering, a wide
// backfill — would hold up the roster if they shared a turn, and a market that just
// opened would then stay unfollowed for as long as the slow half took.
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

// Start hands out the places once straight away and then keeps doing so. The first
// pass is immediate because a system that has just come up during trading hours is
// following nothing, and waiting a full interval to notice would lose the first few
// minutes of the day for no reason.
func (liveFollowRosterJob *LiveFollowRosterJob) Start(executionContext context.Context) {
	go liveFollowRosterJob.run(executionContext)
}

// Stop ends the job after the pass it may be in the middle of.
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
			// A pass that outran the interval leaves a tick already waiting, so a
			// stop arriving at that moment would be picked between at random. Looking
			// again is what makes "take on no further passes" mean it.
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

// refresh writes down only what went wrong. A pass that simply found nothing to
// change stays quiet, because a line every five minutes saying nothing happened is a
// line nobody reads when something does.
func (liveFollowRosterJob *LiveFollowRosterJob) refresh(executionContext context.Context) {
	if refreshError := liveFollowRosterJob.kCandleFollowApplication.
		RefreshFixedFollows(executionContext); refreshError != nil {
		log.Printf("live follow roster did not run: %v", refreshError)
	}
}
