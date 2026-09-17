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

// HistoryWindow covers a stretch somebody named, reaching back from now by the
// lookback they gave.
//
// **It never looks at what is already stored, and that is the whole of what separates
// it from a backfill.** A backfill starts wherever the stored data left off, so a
// minute missing from the *middle* of a stretch is one it never comes back for:
// holding day one and day thirty, it begins after day thirty and the twenty-eight
// days between are gone for good. Asking about the whole stretch is the only way that
// hole ever gets filled.
//
// Asking is not writing. What comes back is stored under whichever rule the run was
// given, and the run that uses this window keeps what it already holds.
//
// The start is rounded down to a bucket edge for the same reason every other
// lookback-derived start is: a stretch beginning mid-bucket makes the oldest bucket
// of every coarseness begin part way through itself, and nothing downstream can tell
// that apart from a market that simply traded little. See BackfillWindow.
func (kCandleIngestionDomain KCandleIngestionDomain) HistoryWindow(
	symbol string, market vo.MarketVo, lookback time.Duration,
) vo.KCandleFetchWindowVo {
	startTime := NewCoarsestAggregationIntervalDomain().BucketStart(
		kCandleIngestionDomain.currentTime.Add(-lookback))

	return vo.NewKCandleFetchWindowVo(
		symbol, market, startTime, kCandleIngestionDomain.LatestClosedOpenTime())
}

// HistoryChunks is the same stretch HistoryWindow covers, cut into one day each,
// oldest first.
//
// **Cutting it up is what makes the length of the stretch stop mattering.** Asked for
// four years in one window, everything the source answers with has to be held at once
// before a single candle is stored — two million of them, which is not a slow request
// but a dead one. A day at a time is fetched, stored and let go of, so four years
// costs exactly what one day costs, over and over.
//
// **Oldest first is deliberate.** A run that dies part way then leaves one continuous
// block of old candles with the gap at the recent end, and that gap is precisely the
// shape the ordinary backfill closes. Newest first would leave the gap in the middle,
// and nothing in this system fills those.
//
// The days are the aggregation bucket's days, not the market's: the point of the cut
// is a bounded amount of work, and a market's own day would make the boundary move
// with the market for no gain. Each chunk ends on the last open time of its day, and
// the final one on the last minute that has actually closed — so the chunks meet end
// to end with no gap and no overlap, and together cover exactly what HistoryWindow
// covers.
func (kCandleIngestionDomain KCandleIngestionDomain) HistoryChunks(
	symbol string, market vo.MarketVo, lookback time.Duration,
) []vo.KCandleFetchWindowVo {
	wholeWindow := kCandleIngestionDomain.HistoryWindow(symbol, market, lookback)
	if wholeWindow.IsEmpty() {
		return []vo.KCandleFetchWindowVo{}
	}

	coarsestInterval := NewCoarsestAggregationIntervalDomain()

	chunks := make([]vo.KCandleFetchWindowVo, 0)
	for chunkStart := wholeWindow.StartTime; !chunkStart.After(wholeWindow.EndTime); {
		chunkEnd := coarsestInterval.BucketStart(chunkStart).
			Add(coarsestInterval.duration).Add(-kCandleIngestionDomain.interval())
		if chunkEnd.After(wholeWindow.EndTime) {
			chunkEnd = wholeWindow.EndTime
		}

		chunks = append(chunks,
			vo.NewKCandleFetchWindowVo(symbol, market, chunkStart, chunkEnd))

		chunkStart = chunkEnd.Add(kCandleIngestionDomain.interval())
	}

	return chunks
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
