package service

import (
	"context"
	"sync"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleSnapshotRelay is the live line of a replica off duty: it reads the snapshots the replica on duty writes, so a market with limited places is followed from the source only once.
// Each minute's snapshot is passed on in order, so a closing candle is never lost to the next one; a snapshot is passed once, and only while recent,
// so when the replica on duty goes silent the line goes silent too and the feed tells viewers it stalled, instead of replaying an old candle as live.
type kCandleSnapshotRelay struct {
	liveKCandleSnapshotRepository _interface.ILiveKCandleSnapshotRepository
	clockProxy                    _interface.IClockProxy
	interval                      time.Duration
	// freshFor is how recent a snapshot must be to count as live; older ones are what an outage left behind.
	freshFor time.Duration

	// passedUntil remembers, per line, the newest sighting already passed on, across reconnects.
	mutex       *sync.Mutex
	passedUntil map[string]time.Time
}

func newKCandleSnapshotRelay(
	liveKCandleSnapshotRepository _interface.ILiveKCandleSnapshotRepository,
	clockProxy _interface.IClockProxy,
	interval time.Duration,
	freshFor time.Duration,
) kCandleSnapshotRelay {
	return kCandleSnapshotRelay{
		liveKCandleSnapshotRepository: liveKCandleSnapshotRepository,
		clockProxy:                    clockProxy,
		interval:                      interval,
		freshFor:                      freshFor,
		mutex:                         &sync.Mutex{},
		passedUntil:                   map[string]time.Time{},
	}
}

func (kCandleSnapshotRelay kCandleSnapshotRelay) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	liveKCandles := make(chan vo.LiveKCandleVo, len(channel.Symbols))

	go func() {
		defer close(liveKCandles)

		ticker := time.NewTicker(kCandleSnapshotRelay.interval)
		defer ticker.Stop()

		for {
			since := kCandleSnapshotRelay.cursorOf(channel.Key)
			snapshots, findError := kCandleSnapshotRelay.liveKCandleSnapshotRepository.FindObservedAfter(
				executionContext, channel.Symbols, since)
			// A read that fails ends the line, so the feed reconnects with its usual back-off.
			if findError != nil {
				return
			}

			for _, snapshot := range snapshots {
				kCandleSnapshotRelay.advance(channel.Key, snapshot.ObservedAt)

				select {
				case liveKCandles <- snapshot.ToLiveKCandleVo():
				case <-executionContext.Done():
					return
				}
			}

			select {
			case <-executionContext.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return liveKCandles, nil
}

// cursorOf is the newest sighting already passed on for this line, or the oldest one still fresh, whichever is later.
// It and advance exist to scope the cursor lock with defer, since lines of the same relay read and move it from their own goroutines.
func (kCandleSnapshotRelay kCandleSnapshotRelay) cursorOf(channelKey string) time.Time {
	freshSince := kCandleSnapshotRelay.clockProxy.Now().Add(-kCandleSnapshotRelay.freshFor)

	kCandleSnapshotRelay.mutex.Lock()
	defer kCandleSnapshotRelay.mutex.Unlock()

	passedUntil := kCandleSnapshotRelay.passedUntil[channelKey]
	if passedUntil.After(freshSince) {
		return passedUntil
	}

	return freshSince
}

// advance records a sighting as passed on.
func (kCandleSnapshotRelay kCandleSnapshotRelay) advance(channelKey string, observedAt time.Time) {
	kCandleSnapshotRelay.mutex.Lock()
	defer kCandleSnapshotRelay.mutex.Unlock()

	if observedAt.After(kCandleSnapshotRelay.passedUntil[channelKey]) {
		kCandleSnapshotRelay.passedUntil[channelKey] = observedAt
	}
}
