package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleIngestionDomain owns every "which candles" rule of automatic ingestion:
// which candle counts as closed, where each of the two stretches worth fetching
// begins, and that a backfill begins where a bucket does.
//
// The periodic round and the startup backfill differ only in the window they ask
// for; everything after that is one shared path. Keeping both window rules on one
// object is what makes that true, and it is why widening the K candle length later
// touches this file and no other.
type KCandleIngestionDomain struct {
	currentTime      time.Time
	roundCandleCount int
	backfillLookback time.Duration
}

// NewKCandleIngestionDomain judges "closed" and "how far back" against currentTime.
// It does not assume the caller already screened either number.
//
// A lookback shorter than one bucket of the coarsest coarseness is raised to it,
// because the two things asked of a backfill cannot both hold below that. Reaching
// back an hour and then down to a bucket edge reaches back up to a day and a half —
// far past what a caller setting an hour was asking for, and the setting exists
// precisely to keep the first round after a long silence small. Above the floor the
// two agree again: the reach is the lookback plus less than one bucket, so never more
// than twice what was asked.
//
// Zero or less is refused rather than raised. Asking to reach back no distance is not
// a small request, it is an incoherent one — and refusing it here is what the round
// candle count already does with the same kind of value.
func NewKCandleIngestionDomain(
	currentTime time.Time,
	roundCandleCount int,
	backfillLookback time.Duration,
) (KCandleIngestionDomain, error) {
	if roundCandleCount <= 0 {
		return KCandleIngestionDomain{}, fmt.Errorf(
			"%w: 單輪取回根數必須大於零", ErrKCandleIngestionValidation)
	}

	if backfillLookback <= 0 {
		return KCandleIngestionDomain{}, fmt.Errorf(
			"%w: 回補上限必須大於零", ErrKCandleIngestionValidation)
	}

	return KCandleIngestionDomain{
		currentTime:      currentTime.UTC(),
		roundCandleCount: roundCandleCount,
		backfillLookback: max(backfillLookback, coarsestBucketSpan()),
	}, nil
}

// coarsestBucketSpan is how long one bucket of the coarsest coarseness covers, which
// is both the floor under the lookback and the most the alignment can add to it.
//
// It is read off the interval set rather than written down for the same reason the
// alignment is: a second copy of "a day" would go out of step the moment a coarser
// interval is added, and nothing would report it.
func coarsestBucketSpan() time.Duration {
	return time.Duration(
		NewCoarsestAggregationIntervalDomain().SourceCandleCount(1)) * KCandleInterval
}

// CurrentTime is the moment every ingestion rule is judged against. It is handed
// out so that a caller applying the K candle rules to a fetched candle judges
// "in the future" against the same moment the window was built from.
func (kCandleIngestionDomain KCandleIngestionDomain) CurrentTime() time.Time {
	return kCandleIngestionDomain.currentTime
}

// LatestClosedOpenTime is the open time of the newest K candle whose interval has
// fully elapsed. The one after it is still running, so its figures would still move.
func (kCandleIngestionDomain KCandleIngestionDomain) LatestClosedOpenTime() time.Time {
	interval := kCandleIngestionDomain.interval()

	return kCandleIngestionDomain.currentTime.Truncate(interval).Add(-interval)
}

// ScheduledWindow covers the newest closed candle and the few before it. More than
// one is deliberate: it absorbs figures the source corrects after the fact, and it
// quietly refills whatever a failed round left behind.
//
// It does not consider whether the market is trading: a round just after the close
// still has that day's last candle to collect. Narrowing the window to what a market
// could actually hold is the market's own job, and it is done to this window
// afterwards.
func (kCandleIngestionDomain KCandleIngestionDomain) ScheduledWindow(
	symbol string, market vo.MarketVo,
) vo.KCandleFetchWindowVo {
	endTime := kCandleIngestionDomain.LatestClosedOpenTime()
	candlesBefore := time.Duration(kCandleIngestionDomain.roundCandleCount-1) *
		kCandleIngestionDomain.interval()

	return vo.NewKCandleFetchWindowVo(symbol, market, endTime.Add(-candlesBefore), endTime)
}

// BackfillWindow covers the gap left behind while nothing was running. A zero
// latestStoredOpenTime means the symbol has never held a K candle, which fills the
// whole lookback. When the gap is already closed the window comes back empty.
//
// **The lookback is how far back it reaches at least, not at most.** Reaching back by
// it lands on an arbitrary minute, and a stretch that begins mid-bucket makes the
// oldest bucket of every coarseness begin part way through itself — merged and handed
// over as a whole one. At one day that is an afternoon's opening price and half a
// day's volume reported as the day's, and nothing downstream can tell that apart from
// a market that simply traded little. So the reach-back is rounded *down* to a bucket
// edge, which costs at most one more bucket's worth of candles.
//
// That cost is paid by every backfill whose start comes from the lookback — a restart
// after being down longer than it, a symbol joining the watchlist, a manual catch-up,
// and every round for a symbol the source never answers for. Only a symbol whose
// stored data is newer than the aligned edge escapes it, which after the first
// successful fetch is the ordinary case.
//
// Down rather than up: rounding up would give away most of a day of the gap it was
// asked to close, and would put the oldest bucket at today.
//
// **Only that start is rounded.** The one derived from stored data already sits on a
// minute edge and abuts the candles that are there; rounding it down would step back
// over them and fetch what is already stored. Which is also why the comparison below
// needs no special case: rounding only moves a start earlier, so a stored-data start
// that used to win still wins.
func (kCandleIngestionDomain KCandleIngestionDomain) BackfillWindow(
	symbol string,
	market vo.MarketVo,
	latestStoredOpenTime time.Time,
) vo.KCandleFetchWindowVo {
	startTime := NewCoarsestAggregationIntervalDomain().BucketStart(
		kCandleIngestionDomain.currentTime.Add(-kCandleIngestionDomain.backfillLookback))

	if !latestStoredOpenTime.IsZero() {
		nextAfterStored := latestStoredOpenTime.UTC().Add(kCandleIngestionDomain.interval())
		if nextAfterStored.After(startTime) {
			startTime = nextAfterStored
		}
	}

	return vo.NewKCandleFetchWindowVo(
		symbol, market, startTime, kCandleIngestionDomain.LatestClosedOpenTime())
}

// SelectClosed drops any candle the source handed over whose interval has not
// finished yet, however the source chose to report it.
func (kCandleIngestionDomain KCandleIngestionDomain) SelectClosed(
	marketKCandles []vo.MarketKCandleVo,
) []vo.MarketKCandleVo {
	latestClosedOpenTime := kCandleIngestionDomain.LatestClosedOpenTime()

	closedKCandles := make([]vo.MarketKCandleVo, 0, len(marketKCandles))
	for _, marketKCandle := range marketKCandles {
		if !marketKCandle.OpenTime.UTC().After(latestClosedOpenTime) {
			closedKCandles = append(closedKCandles, marketKCandle)
		}
	}

	return closedKCandles
}

// RoundCoverage is how much time one scheduled round asks about.
//
// It is what makes an empty answer worth interpreting: a round that asked about the
// last five minutes and heard nothing has said something, and a market that has only
// been open for two of them has not.
func (kCandleIngestionDomain KCandleIngestionDomain) RoundCoverage() time.Duration {
	return time.Duration(kCandleIngestionDomain.roundCandleCount) * kCandleIngestionDomain.interval()
}

// interval is how long one K candle covers, taken from the single place the
// project writes that length down.
func (kCandleIngestionDomain KCandleIngestionDomain) interval() time.Duration {
	return KCandleInterval
}
