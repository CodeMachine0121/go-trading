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
//
// It asks whether a stretch holds the moment rather than checking the moment's own
// weekday first, and that is not a tidier way of writing the same thing: a stretch
// that runs past midnight trades into a day the market never opens on. Friday's
// evening board is still trading at two on Saturday morning, and a weekday check
// would call it shut.
func (marketDomain MarketDomain) IsOpen(moment time.Time) bool {
	if marketDomain.neverCloses() {
		return true
	}

	isOpen := false
	marketDomain.eachSessionOccurrence(moment, moment,
		func(occurrence vo.TradingSessionOccurrenceVo) {
			if occurrence.Contains(moment) {
				isOpen = true
			}
		})

	return isOpen
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
// It counts each stretch in turn and **never counts a bucket twice**, because a venue
// may trade more than once a day and two of its stretches can meet inside one bucket:
// Taiwan index futures shuts its day board at 13:45 and opens its evening board at
// 15:00, which in a four-hour bucket is the same bucket twice. Summing the two would
// report one bar more than the chart has, on a path where nothing anywhere raises an
// error. So each stretch starts counting after the last bucket already counted.
//
// That correction relies on the stretches arriving in start order, which
// MarketCatalogDomain guarantees. It also only merges a bucket with the stretch
// immediately before it — enough while two stretches a day is the most any market
// here has, because the coarsest bucket is a day and the shortest gap between
// stretches is over an hour. **Read this before adding a third stretch to a day.**
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
	lastCountedBucketStart := time.Time{}
	marketDomain.eachTradableRange(startTime, endTime,
		func(rangeStart time.Time, rangeEnd time.Time) {
			firstBucketStart := bucketStartOf(rangeStart, bucketDuration)
			lastBucketStart := bucketStartOf(rangeEnd, bucketDuration)

			// Start after the last bucket already counted, so a bucket two stretches
			// share is one bar rather than two.
			if !lastCountedBucketStart.IsZero() && !firstBucketStart.After(lastCountedBucketStart) {
				firstBucketStart = lastCountedBucketStart.Add(bucketDuration)
			}

			if lastBucketStart.Before(firstBucketStart) {
				return
			}

			tradingBucketCount += int(lastBucketStart.Sub(firstBucketStart)/bucketDuration) + 1
			lastCountedBucketStart = lastBucketStart
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
	marketDomain.eachTradableRange(startTime, endTime,
		func(_ time.Time, _ time.Time) {
			holdsTrading = true
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

// SessionOccurrenceAt is the stretch of trading a round running at this moment is
// working on: the one holding the moment, or — once that one has shut — the one that
// has already run today.
//
// The second clause is what keeps the round just after the closing bell working on the
// stretch that just ended rather than on nothing. At 13:46 the day board is over, but
// its last candle is still what this round is fetching and a decision that it was shut
// today is still the decision that applies. A stretch that ended before this day began
// does not qualify: yesterday is not what this moment is about.
//
// It is one question rather than two ("which stretch is running" plus "which one just
// finished") because every caller wants the same thing — the stretch this moment is
// about — and splitting it would leave each of them to remember the closing-bell case.
//
// A market that never closes has no stretches and answers with none.
func (marketDomain MarketDomain) SessionOccurrenceAt(
	moment time.Time,
) vo.TradingSessionOccurrenceVo {
	if marketDomain.neverCloses() {
		return vo.TradingSessionOccurrenceVo{}
	}

	dayStart := marketDomain.localMidnightOf(
		moment.In(marketDomain.rules.TradingSession.Location)).UTC()

	occurrenceAt := vo.TradingSessionOccurrenceVo{}
	marketDomain.eachSessionOccurrence(moment.Add(-occurrenceLookback), moment,
		func(occurrence vo.TradingSessionOccurrenceVo) {
			if occurrence.Contains(moment) {
				occurrenceAt = occurrence

				return
			}

			if occurrence.EndTime.After(moment) || occurrence.EndTime.Before(dayStart) {
				return
			}

			occurrenceAt = occurrence
		})

	return occurrenceAt
}

// occurrenceLookback is how far back SessionOccurrenceAt looks for the stretch a
// moment belongs to. A stretch may start the day before the one it trades into, so a
// day would not be enough; two is, and the walk is bounded by its own length.
const occurrenceLookback = 2 * oneCalendarDay

// SessionElapsedAt is how much of this market's current stretch of trading is already
// behind it at this moment: nothing before the bell, the whole stretch once it has run.
//
// It answers "has this market had a chance to say anything yet". A market with no
// hours always has: it has been trading all along.
func (marketDomain MarketDomain) SessionElapsedAt(moment time.Time) time.Duration {
	if marketDomain.neverCloses() {
		return marketDomain.sinceLocalMidnight(moment.UTC())
	}

	occurrence := marketDomain.SessionOccurrenceAt(moment)
	if occurrence.IsZero() {
		return 0
	}

	// The occurrence is either holding this moment or already over by it, so the
	// elapsed time is never negative and nothing here has to say what that would mean.
	elapsed := moment.Sub(occurrence.StartTime)
	if length := occurrence.Length(); elapsed > length {
		return length
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
//
// An offset past twenty-four hours is how a stretch that crosses midnight is written,
// and it needs no special case here: an hour reading of twenty-nine is normalised into
// five in the morning of the following day, in that market's own zone.
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
// window that a stretch of trading could actually hold.
//
// It walks the days the window touches rather than the stretches, which is what bounds
// the work: however long the window is, the search is its own length and no longer, so
// a holiday of any length costs nothing extra and needs no list of holidays to skip.
func (marketDomain MarketDomain) overlappingCandleOpenTimes(
	window vo.KCandleFetchWindowVo,
) (time.Time, time.Time, bool) {
	earliestOpenTime := time.Time{}
	latestOpenTime := time.Time{}

	marketDomain.eachTradableRange(window.StartTime, window.EndTime,
		func(rangeStart time.Time, rangeEnd time.Time) {
			if earliestOpenTime.IsZero() {
				earliestOpenTime = rangeStart
			}

			latestOpenTime = rangeEnd
		})

	return earliestOpenTime, latestOpenTime, !earliestOpenTime.IsZero()
}

// eachTradableRange walks a stretch of time and hands the visitor each run of candle
// open times inside it that this market could actually have traded, earliest first.
//
// It is the walk almost every question about a stretch of time wants: how many buckets
// hold trading, which candle open times a window could cover, whether any trading
// happened at all. Each of them used to work the overlap out for itself and then check
// whether there was one, which is three copies of the same reading of a session's
// edges — and the copy that was not updated would keep answering.
//
// The last open time is the one before the closing bell rather than the bell itself: a
// candle stamped at the bell would cover time the market was already shut for. A run
// that holds no open time at all is not handed over, so a visitor never has to ask.
//
// It is deliberately not what "is the market open" reads. A moment thirty seconds
// before the bell is inside the session and after its last candle open time, so this
// walk would call it shut — which is why IsOpen asks about occurrences instead.
func (marketDomain MarketDomain) eachTradableRange(
	startTime time.Time,
	endTime time.Time,
	visit func(rangeStart time.Time, rangeEnd time.Time),
) {
	marketDomain.eachSessionOccurrence(startTime, endTime,
		func(occurrence vo.TradingSessionOccurrenceVo) {
			rangeStart := startTime
			if occurrence.StartTime.After(rangeStart) {
				rangeStart = occurrence.StartTime
			}

			rangeEnd := endTime
			if lastOpenTime := occurrence.EndTime.Add(-KCandleInterval); lastOpenTime.Before(rangeEnd) {
				rangeEnd = lastOpenTime
			}

			if rangeEnd.Before(rangeStart) {
				return
			}

			visit(rangeStart, rangeEnd)
		})
}

// eachSessionOccurrence walks the market's own opening days across a stretch of time
// and hands the visitor every stretch of trading those days hold, in start order.
//
// It is the single place that knows which days this market opens on, when each of its
// stretches runs, and which business day each one's trading counts towards — so a
// venue that grows another board is a change here and nowhere else. Two copies of that
// walk would go out of step, and the one that was not updated would keep answering.
//
// It begins a day earlier than it was asked to, because a stretch that runs past
// midnight starts on the day before the one it trades into. Without that extra day,
// Friday's evening board would be invisible to anything asking about Saturday morning
// — and Saturday is not a day this market opens on, so nothing else would find it.
func (marketDomain MarketDomain) eachSessionOccurrence(
	startTime time.Time,
	endTime time.Time,
	visit func(occurrence vo.TradingSessionOccurrenceVo),
) {
	location := marketDomain.rules.TradingSession.Location
	lastLocalDay := marketDomain.localMidnightOf(endTime.In(location))
	firstLocalDay := marketDomain.localMidnightOf(startTime.In(location)).AddDate(0, 0, -1)

	for localDay := firstLocalDay; !localDay.After(lastLocalDay); localDay = localDay.AddDate(0, 0, 1) {
		if !marketDomain.tradesOn(localDay.Weekday()) {
			continue
		}

		for _, tradingStretch := range marketDomain.rules.TradingSession.Stretches {
			visit(vo.NewTradingSessionOccurrenceVo(
				marketDomain.sessionMomentOn(localDay, tradingStretch.StartOffset),
				marketDomain.sessionMomentOn(localDay, tradingStretch.EndOffset),
				marketDomain.businessDateOf(localDay, tradingStretch),
			))
		}
	}
}

// businessDateOf is the business day the trading of a stretch beginning on this local
// day counts towards: that day, or the next day this market opens on.
//
// An evening board's trading counts towards the next business day — the reading the
// exchange itself takes — so Friday evening belongs to Monday rather than to a
// Saturday nobody trades. Whether a stretch reads that way is written on the stretch,
// so this is arithmetic about a calendar and not a rule about evenings.
func (marketDomain MarketDomain) businessDateOf(
	localDay time.Time, tradingStretch vo.TradingStretchVo,
) time.Time {
	if !tradingStretch.BelongsToNextBusinessDay {
		return localDay.UTC()
	}

	return marketDomain.nextOpeningDayAfter(localDay).UTC()
}

// nextOpeningDayAfter is the next day this market opens on.
//
// It is only ever asked about a day the market itself opens on — the walk skips every
// other day before reaching this — so at least one day of the week opens and the
// search always finds one. The week bounds it anyway rather than trusting that: a
// misconfigured market answers with a day a week out, which is wrong and visible,
// instead of never answering at all.
func (marketDomain MarketDomain) nextOpeningDayAfter(localDay time.Time) time.Time {
	nextDay := localDay
	for dayCount := 1; dayCount <= daysInWeek; dayCount++ {
		nextDay = localDay.AddDate(0, 0, dayCount)
		if marketDomain.tradesOn(nextDay.Weekday()) {
			break
		}
	}

	return nextDay
}

// daysInWeek bounds the search for a market's next opening day.
const daysInWeek = 7

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
