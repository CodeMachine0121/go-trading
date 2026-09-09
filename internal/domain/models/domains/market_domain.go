package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketDomain is one market and everything the rest of the system needs to know
// about how it behaves: whether it is trading right now, which part of a stretch of
// time could possibly hold candles, how many of its symbols may be followed live at
// once, and which of its own days a moment belongs to.
//
// The four answers live together because they change together. Opening hours,
// follow ceiling and calendar are one fact about a venue; splitting them would put
// the same "which market is this" question in four places and give it four chances
// to be answered differently.
//
// A round-the-clock market is not a special case here — it is the market whose
// session never closes, and every method reads it through the same rules. That is
// what keeps `if market == taiwanStock` out of the rest of the codebase entirely.
type MarketDomain struct {
	value vo.MarketVo
	rules vo.MarketRulesVo
}

// Value is the market this is, as it is stored and routed on.
func (marketDomain MarketDomain) Value() vo.MarketVo {
	return marketDomain.value
}

// IsOpen reports whether the market is trading at this moment.
//
// It answers "can this be followed live" and "should the console say it is closed".
// It deliberately does not answer "is there anything worth fetching" — a round
// running at 13:33 still has the 13:29 candle to collect, and conflating the two
// would lose it. ClampToTradingSession answers that one.
func (marketDomain MarketDomain) IsOpen(moment time.Time) bool {
	if marketDomain.neverCloses() {
		return true
	}

	localMoment := moment.In(marketDomain.rules.TradingSession.Location)
	if !marketDomain.tradesOn(localMoment.Weekday()) {
		return false
	}

	sinceMidnight := marketDomain.sinceLocalMidnight(localMoment)

	return sinceMidnight >= marketDomain.rules.TradingSession.DailyStart &&
		sinceMidnight < marketDomain.rules.TradingSession.DailyEnd
}

// ClampToTradingSession narrows a fetch window to the part of it that could hold
// candles at all, and comes back empty when none of it could.
//
// This is the whole of "a closed market is not a gap". A window covering a night, a
// weekend or a holiday shrinks to nothing, and an empty window already means "there
// is nothing to do" everywhere it is read — so skipping a closed market needs no new
// branch anywhere. It is also why the round that runs just after the close still
// collects the day's last candle: the window still overlaps the session even though
// the market no longer does.
//
// A market that never closes gets its window back untouched.
func (marketDomain MarketDomain) ClampToTradingSession(
	window vo.KCandleFetchWindowVo,
) vo.KCandleFetchWindowVo {
	if marketDomain.neverCloses() || window.IsEmpty() {
		return window
	}

	earliestOpenTime, latestOpenTime, hasOverlap := marketDomain.overlappingCandleOpenTimes(window)
	if !hasOverlap {
		return marketDomain.emptyWindow(window)
	}

	return vo.NewKCandleFetchWindowVo(
		window.Symbol, window.Market, earliestOpenTime, latestOpenTime)
}

// TradingBucketCountBetween is how many buckets of the given length, inside the
// stretch, could hold any trading at all. It is what turns a stretch into a number of
// candles, and it does so by **counting buckets rather than dividing time**.
//
// The difference is the whole reason this exists. Dividing the trading time by the
// bucket length reads as though it should work — a market that trades four and a half
// hours a day offers that much of a day's clock — but a bucket is not a quantity of
// time, it is a slot with edges. A bucket that catches one minute of trading is a
// whole candle, exactly like one that catches all of it. So the division undercounts
// wherever a bucket is long enough to reach past a session's edges, and it undercounts
// worse the coarser the bucket: a Taiwan trading day is five hourly buckets rather
// than four, two four-hour buckets rather than one, and five Taiwan days are five
// daily buckets rather than the fifth of one the division reports.
//
// It counts arithmetically rather than visiting each bucket, and that is not a
// micro-optimisation: nothing bounds the stretch a caller may name, so visiting them
// would cost one map entry per bucket — a century at a minute a bucket is nine million
// entries and most of a gigabyte, spent *before* the ceiling that would have refused
// the range is consulted.
//
// It sums each session's buckets without checking whether two of them share one,
// because a venue is written down with one session a day and the coarsest bucket is a
// day: two sessions can never meet in the same bucket. The day that stops being true
// is the day a second daily session is added, and the note about it lives on the walk
// that would have to change — not here, where it would be a line no input reaches.
//
// **A market that never closes divides**, and that is a known inconsistency rather
// than an oversight: it answers the way every count in this system has always answered
// for such a market, which is one short of the buckets an inclusive range can actually
// produce. Correcting it moves every count on that path and every bar the user sees, so
// it is written down as its own change rather than smuggled in with this one.
func (marketDomain MarketDomain) TradingBucketCountBetween(
	startTime time.Time, endTime time.Time, bucketDuration time.Duration,
) int {
	if marketDomain.neverCloses() {
		return int(endTime.Sub(startTime) / bucketDuration)
	}

	tradingBucketCount := 0
	marketDomain.eachTradingDaySession(startTime, endTime,
		func(sessionStart time.Time, sessionEnd time.Time) {
			overlapStart := startTime
			if sessionStart.After(overlapStart) {
				overlapStart = sessionStart
			}

			// The last open time a session can hold, not the moment it shuts: a candle
			// stamped at the closing bell would cover time the market was closed for,
			// which is the same reading ClampToTradingSession takes.
			overlapEnd := endTime
			if sessionLastOpenTime := sessionEnd.Add(-KCandleInterval); sessionLastOpenTime.Before(overlapEnd) {
				overlapEnd = sessionLastOpenTime
			}

			if overlapEnd.Before(overlapStart) {
				return
			}

			firstBucketStart := bucketStartOf(overlapStart, bucketDuration)
			lastBucketStart := bucketStartOf(overlapEnd, bucketDuration)
			tradingBucketCount += int(lastBucketStart.Sub(firstBucketStart)/bucketDuration) + 1
		})

	return tradingBucketCount
}

// HoldsTrading reports whether the market is open at any point of the stretch.
//
// It is asked out loud rather than derived from a bucket count being zero, because a
// count answers a different question and rounds: a thirty-second stretch of a market
// that never shuts holds trading throughout and yet holds no whole minute-bucket, so
// reading the count as the answer would call a round-the-clock market closed. That
// contradiction is one this system says out loud it cannot have.
func (marketDomain MarketDomain) HoldsTrading(startTime time.Time, endTime time.Time) bool {
	if marketDomain.neverCloses() {
		return endTime.After(startTime)
	}

	holdsTrading := false
	marketDomain.eachTradingDaySession(startTime, endTime,
		func(sessionStart time.Time, sessionEnd time.Time) {
			overlapStart := startTime
			if sessionStart.After(overlapStart) {
				overlapStart = sessionStart
			}

			overlapEnd := endTime
			if sessionLastOpenTime := sessionEnd.Add(-KCandleInterval); sessionLastOpenTime.Before(overlapEnd) {
				overlapEnd = sessionLastOpenTime
			}

			if !overlapEnd.Before(overlapStart) {
				holdsTrading = true
			}
		})

	return holdsTrading
}

// SimultaneousFollowCeiling is how many of this market's symbols may be followed
// live at the same time. Zero means the market data plan sets no ceiling.
//
// It is worked out rather than stored: as many channels as the plan opens, each
// carrying as many symbols as the plan allows on one. A plan is sold in those two
// numbers, so those two are what is set — and a ceiling that contradicts them
// becomes a thing nobody can write down.
func (marketDomain MarketDomain) SimultaneousFollowCeiling() int {
	if marketDomain.rules.SimultaneousChannelCeiling <= 0 {
		return 0
	}

	return marketDomain.rules.SimultaneousChannelCeiling * marketDomain.SymbolsPerLiveChannel()
}

// SymbolsPerLiveChannel is how many trading symbols one of this market's live
// channels may carry, which is never fewer than one: a channel carrying nothing is
// not a channel. A market whose source follows symbols one at a time therefore needs
// no setting at all.
func (marketDomain MarketDomain) SymbolsPerLiveChannel() int {
	if marketDomain.rules.SymbolsPerLiveChannel <= 0 {
		return 1
	}

	return marketDomain.rules.SymbolsPerLiveChannel
}

// HasFollowCeiling reports a market that limits how many of its symbols may be
// followed live at once, and therefore hands its places out from a roster rather
// than to whoever looks first.
//
// It is asked separately from whether the market closes, because the two are
// different facts that happen to coincide today. A venue could publish round the
// clock and still cap how many feeds one plan may open, and reading one as the other
// would then hand out places nobody was allowed to take.
func (marketDomain MarketDomain) HasFollowCeiling() bool {
	return marketDomain.rules.SimultaneousChannelCeiling > 0
}

// oneCalendarDay is the length of a market's own day, for a market that has no local
// day of its own to speak of.
//
// It is named so that it is not mistaken for a bucket length. A daily aggregation
// bucket happens to be the same duration, and asking for that one goes through
// NewCoarsestAggregationIntervalDomain — see the note below on why these two must
// not be folded together.
const oneCalendarDay = 24 * time.Hour

// TradingDateOf is the market's own calendar day a moment falls on — midnight local,
// expressed universally.
//
// It exists so that "we already decided this market is closed today" survives until
// the market's own tomorrow, not until midnight somewhere else. A round at 23:00 in
// Taipei is the same trading day as one at 10:00; a round at 23:00 in universal time
// is already the next one.
//
// **This is not a bucket edge**, however alike the two look for a market that never
// closes. A daily aggregation bucket is cut from midnight in universal time for every
// market; a trading date is midnight in the market's own zone, which for Taipei is
// 16:00 the previous day in universal time — nowhere near a bucket edge. They agree
// only for a round-the-clock market, and only by coincidence. Anything wanting the
// edge asks NewCoarsestAggregationIntervalDomain; do not fold these two together.
func (marketDomain MarketDomain) TradingDateOf(moment time.Time) time.Time {
	if marketDomain.neverCloses() {
		return moment.UTC().Truncate(oneCalendarDay)
	}

	localMoment := moment.In(marketDomain.rules.TradingSession.Location)

	return time.Date(
		localMoment.Year(), localMoment.Month(), localMoment.Day(),
		0, 0, 0, 0, marketDomain.rules.TradingSession.Location,
	).UTC()
}

// NeverCloses reports a market that trades round the clock.
//
// It is asked out loud because a market with no hours also has no days off: deciding
// that such a market is "shut for the day" is not a conclusion about the world but a
// misreading of a quiet stretch, and one quiet stretch would then stop it being
// fetched until tomorrow.
func (marketDomain MarketDomain) NeverCloses() bool {
	return marketDomain.neverCloses()
}

// neverCloses reads the one shape a session takes when there is no session: a market
// with no zone to say its hours in has no hours.
func (marketDomain MarketDomain) neverCloses() bool {
	return marketDomain.rules.TradingSession.Location == nil
}

func (marketDomain MarketDomain) tradesOn(weekday time.Weekday) bool {
	for _, tradingWeekday := range marketDomain.rules.TradingSession.Weekdays {
		if tradingWeekday == weekday {
			return true
		}
	}

	return false
}

// SessionElapsedAt is how much of this market's session is already behind it at this
// moment: nothing before the bell, the whole session once it has rung.
//
// It answers "has this market had a chance to say anything yet". A market with no
// hours always has: it has been trading all along.
//
// It reads the clock fields rather than subtracting from midnight, for the same
// reason IsOpen does — a day is not always twenty-four hours long.
func (marketDomain MarketDomain) SessionElapsedAt(moment time.Time) time.Duration {
	if marketDomain.neverCloses() {
		return marketDomain.sinceLocalMidnight(moment.UTC())
	}

	session := marketDomain.rules.TradingSession
	localMoment := moment.In(session.Location)
	if !marketDomain.tradesOn(localMoment.Weekday()) {
		return 0
	}

	elapsed := marketDomain.sinceLocalMidnight(localMoment) - session.DailyStart
	if elapsed < 0 {
		return 0
	}

	if fullSession := session.DailyEnd - session.DailyStart; elapsed > fullSession {
		return fullSession
	}

	return elapsed
}

// sessionMomentOn is the moment a given point of a market's session falls on, on a
// given local day.
//
// It builds the clock reading rather than adding a stretch of time to midnight, for
// the same reason IsOpen reads clock fields: on a day that gains or loses an hour,
// "nine in the morning" and "nine hours after midnight" are different moments — and
// this file would then answer "when does this market trade today" two ways.
//
// Nothing in Taipei turns on it. But the zone is a setting, and the next market's
// might.
func (marketDomain MarketDomain) sessionMomentOn(
	localDay time.Time, sinceMidnight time.Duration,
) time.Time {
	return time.Date(
		localDay.Year(), localDay.Month(), localDay.Day(),
		int(sinceMidnight/time.Hour), int((sinceMidnight%time.Hour)/time.Minute), 0, 0,
		marketDomain.rules.TradingSession.Location,
	).UTC()
}

// sinceLocalMidnight is how far into its own day a moment is. Reading the clock
// fields rather than subtracting midnight keeps it right on days that are not
// twenty-four hours long.
func (marketDomain MarketDomain) sinceLocalMidnight(localMoment time.Time) time.Duration {
	return time.Duration(localMoment.Hour())*time.Hour +
		time.Duration(localMoment.Minute())*time.Minute +
		time.Duration(localMoment.Second())*time.Second
}

// overlappingCandleOpenTimes reports the first and last candle open time inside the
// window that a session could actually hold.
//
// It walks the days the window touches rather than the sessions, which is what bounds
// the work: however long the window is, the search is its own length and no longer, so
// a holiday of any length costs nothing extra and needs no list of holidays to skip.
func (marketDomain MarketDomain) overlappingCandleOpenTimes(
	window vo.KCandleFetchWindowVo,
) (time.Time, time.Time, bool) {
	earliestOpenTime := time.Time{}
	latestOpenTime := time.Time{}

	marketDomain.eachTradingDaySession(window.StartTime, window.EndTime,
		func(sessionStart time.Time, sessionEnd time.Time) {
			// The last open time a session can hold, not the moment it shuts: a candle
			// stamped at the closing bell would cover time the market was closed for.
			sessionLastOpenTime := sessionEnd.Add(-KCandleInterval)
			if sessionLastOpenTime.Before(window.StartTime) || sessionStart.After(window.EndTime) {
				return
			}

			if earliestOpenTime.IsZero() {
				earliestOpenTime = sessionStart
				if window.StartTime.After(sessionStart) {
					earliestOpenTime = window.StartTime
				}
			}

			latestOpenTime = sessionLastOpenTime
			if window.EndTime.Before(sessionLastOpenTime) {
				latestOpenTime = window.EndTime
			}
		})

	return earliestOpenTime, latestOpenTime, !earliestOpenTime.IsZero()
}

// eachTradingDaySession walks the market's own days from one moment to another and
// hands each trading day's session to the visitor, as the two moments it runs between.
//
// It is the single place that knows which days this market trades and when its
// session runs, so a venue that grows a second daily session — an afternoon board, an
// evening board — is a change here and nowhere else. Two copies of that walk would go
// out of step, and the one that was not updated would keep answering.
//
// **Whoever adds that second session: read TradingBucketCountBetween before you do.**
// It sums each session's buckets and never asks whether two sessions met inside one,
// which is safe only while there is one session a day. With a morning and an afternoon
// board, a day a candle would be counted twice — and the symptom is a number that is
// merely too big, reported by nothing.
func (marketDomain MarketDomain) eachTradingDaySession(
	startTime time.Time,
	endTime time.Time,
	visit func(sessionStart time.Time, sessionEnd time.Time),
) {
	location := marketDomain.rules.TradingSession.Location
	lastLocalDay := marketDomain.localMidnightOf(endTime.In(location))

	for localDay := marketDomain.localMidnightOf(startTime.In(location)); !localDay.After(lastLocalDay); localDay = localDay.AddDate(0, 0, 1) {
		if !marketDomain.tradesOn(localDay.Weekday()) {
			continue
		}

		visit(
			marketDomain.sessionMomentOn(localDay, marketDomain.rules.TradingSession.DailyStart),
			marketDomain.sessionMomentOn(localDay, marketDomain.rules.TradingSession.DailyEnd),
		)
	}
}

// localMidnightOf is the start of the local day a moment belongs to. Building the
// date from its parts rather than truncating is what keeps it a local midnight on
// zones whose offset is not a whole number of hours.
func (marketDomain MarketDomain) localMidnightOf(localMoment time.Time) time.Time {
	return time.Date(
		localMoment.Year(), localMoment.Month(), localMoment.Day(),
		0, 0, 0, 0, marketDomain.rules.TradingSession.Location,
	)
}

// emptyWindow is a window covering nothing, said in the one way the rest of the
// system already reads as "there is nothing to do here".
func (marketDomain MarketDomain) emptyWindow(window vo.KCandleFetchWindowVo) vo.KCandleFetchWindowVo {
	return vo.NewKCandleFetchWindowVo(
		window.Symbol, window.Market, window.EndTime.Add(KCandleInterval), window.EndTime)
}
