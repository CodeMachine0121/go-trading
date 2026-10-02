package service

import (
	"context"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleSnapshotRelay is the live line of a replica off duty: it reads the snapshots the replica on duty writes, so a market with limited places is followed from the source only once.
// It reports a candle whenever the replica on duty heard about it again, and goes silent when that replica does, which the feed then tells viewers is a stall.
type kCandleSnapshotRelay struct {
	liveKCandleSnapshotRepository _interface.ILiveKCandleSnapshotRepository
	interval                      time.Duration
}

func (kCandleSnapshotRelay kCandleSnapshotRelay) FollowKCandles(
	executionContext context.Context, channel vo.LiveFollowChannelVo,
) (<-chan vo.LiveKCandleVo, error) {
	liveKCandles := make(chan vo.LiveKCandleVo, len(channel.Symbols))

	go func() {
		defer close(liveKCandles)

		lastObservedAt := map[string]time.Time{}
		ticker := time.NewTicker(kCandleSnapshotRelay.interval)
		defer ticker.Stop()

		for {
			snapshots, findError := kCandleSnapshotRelay.liveKCandleSnapshotRepository.FindBySymbols(
				executionContext, channel.Symbols)
			// A read that fails ends the line, so the feed reconnects with its usual back-off.
			if findError != nil {
				return
			}

			for _, snapshot := range snapshots {
				if !snapshot.ObservedAt.After(lastObservedAt[snapshot.Symbol]) {
					continue
				}
				lastObservedAt[snapshot.Symbol] = snapshot.ObservedAt

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
