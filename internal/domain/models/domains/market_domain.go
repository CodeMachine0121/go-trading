package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketDomain answers every market-behaviour question (trading hours, fetchable stretches, follow ceiling, local calendar) through one set of rules, so no caller branches on a specific market.
type MarketDomain struct {
	value vo.MarketVo
	rules vo.MarketRulesVo
}

func (marketDomain MarketDomain) Value() vo.MarketVo {
	return marketDomain.value
}

// IsOpen reports whether the market is trading at this moment; whether a window still has candles to fetch after the close is ClampToTradingSession's question.
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

// ClampToTradingSession narrows a fetch window to what a session could hold, returning an empty window for nights, weekends and holidays so closed markets need no special branch.
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

// TradingBucketCountBetween counts buckets that touch any trading rather than dividing trading time, since a bucket catching one minute is still a whole candle; it counts arithmetically so huge ranges cost nothing before the ceiling check.
// It assumes one session per day, and for never-closing markets it knowingly divides, answering one short of an inclusive range for compatibility.
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

			// The last open time a session can hold, since a candle stamped at the closing bell covers closed time.
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

// TradingKCandleCountBetween is the expected candle count with both ends included, used to skip fetching already-complete stretches; unlike TradingBucketCountBetween it is inclusive for never-closing markets too.
func (marketDomain MarketDomain) TradingKCandleCountBetween(
	startTime time.Time, endTime time.Time,
) int {
	if !marketDomain.neverCloses() {
		return marketDomain.TradingBucketCountBetween(startTime, endTime, KCandleInterval)
	}

	if endTime.Before(startTime) {
		return 0
	}

	return int(endTime.Sub(startTime)/KCandleInterval) + 1
}

// HoldsTrading is asked directly rather than read off a bucket count, which rounds and would call a sub-minute stretch of a 24/7 market closed.
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

// SimultaneousFollowCeiling is channels times symbols per channel from the data plan, zero meaning no ceiling.
func (marketDomain MarketDomain) SimultaneousFollowCeiling() int {
	if marketDomain.rules.SimultaneousChannelCeiling <= 0 {
		return 0
	}

	return marketDomain.rules.SimultaneousChannelCeiling * marketDomain.SymbolsPerLiveChannel()
}

// SymbolsPerLiveChannel is never fewer than one, so one-at-a-time sources need no setting.
func (marketDomain MarketDomain) SymbolsPerLiveChannel() int {
	if marketDomain.rules.SymbolsPerLiveChannel <= 0 {
		return 1
	}

	return marketDomain.rules.SymbolsPerLiveChannel
}

// HasFollowCeiling reports only whether live follows are capped; whether the market follows a roster is FollowsFixedRoster's question.
func (marketDomain MarketDomain) HasFollowCeiling() bool {
	return marketDomain.rules.SimultaneousChannelCeiling > 0
}

// FollowsFixedRoster reports a market followed for every watched symbol regardless of viewers, rather than on demand when a chart opens.
func (marketDomain MarketDomain) FollowsFixedRoster() bool {
	return marketDomain.rules.FollowsFixedRoster
}

// oneCalendarDay is a never-closing market's day length, deliberately not a bucket length.
const oneCalendarDay = 24 * time.Hour

// TradingDateOf is local midnight of the market's own day, expressed in UTC, so a "closed today" decision lasts until the market's own tomorrow.
// It is not a daily bucket edge (those are UTC midnight); use NewCoarsestAggregationIntervalDomain for that.
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

// Zone is nil for a never-closing market and exists only for sources whose addresses need a local date spelled out; other market questions belong on this model.
func (marketDomain MarketDomain) Zone() *time.Location {
	return marketDomain.rules.TradingSession.Location
}

// NeverCloses matters because a 24/7 market must never be concluded "shut for the day" from a quiet stretch.
func (marketDomain MarketDomain) NeverCloses() bool {
	return marketDomain.neverCloses()
}

// neverCloses treats a session without a zone as having no hours.
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

// SessionElapsedAt is how much of today's session has passed, clamped to [0, full session]; it reads clock fields because days are not always 24 hours.
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

// sessionMomentOn builds the local clock reading rather than adding to midnight, so DST days stay correct.
func (marketDomain MarketDomain) sessionMomentOn(
	localDay time.Time, sinceMidnight time.Duration,
) time.Time {
	return time.Date(
		localDay.Year(), localDay.Month(), localDay.Day(),
		int(sinceMidnight/time.Hour), int((sinceMidnight%time.Hour)/time.Minute), 0, 0,
		marketDomain.rules.TradingSession.Location,
	).UTC()
}

// sinceLocalMidnight reads clock fields so it stays right on days that are not 24 hours long.
func (marketDomain MarketDomain) sinceLocalMidnight(localMoment time.Time) time.Duration {
	return time.Duration(localMoment.Hour())*time.Hour +
		time.Duration(localMoment.Minute())*time.Minute +
		time.Duration(localMoment.Second())*time.Second
}

// overlappingCandleOpenTimes returns the first and last candle open times a session in the window could hold, walking days so holidays cost nothing extra.
func (marketDomain MarketDomain) overlappingCandleOpenTimes(
	window vo.KCandleFetchWindowVo,
) (time.Time, time.Time, bool) {
	earliestOpenTime := time.Time{}
	latestOpenTime := time.Time{}

	marketDomain.eachTradingDaySession(window.StartTime, window.EndTime,
		func(sessionStart time.Time, sessionEnd time.Time) {
			// The last open time a session can hold, since a candle stamped at the closing bell covers closed time.
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

// eachTradingDaySession is the single place that knows trading days and session hours.
// Adding a second daily session would break TradingBucketCountBetween, which assumes sessions never share a bucket.
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

// localMidnightOf builds the date from parts rather than truncating, which stays correct for zones with non-whole-hour offsets.
func (marketDomain MarketDomain) localMidnightOf(localMoment time.Time) time.Time {
	return time.Date(
		localMoment.Year(), localMoment.Month(), localMoment.Day(),
		0, 0, 0, 0, marketDomain.rules.TradingSession.Location,
	)
}

// emptyWindow is the window shape the rest of the system reads as "nothing to do".
func (marketDomain MarketDomain) emptyWindow(window vo.KCandleFetchWindowVo) vo.KCandleFetchWindowVo {
	return vo.NewKCandleFetchWindowVo(
		window.Symbol, window.Market, window.EndTime.Add(KCandleInterval), window.EndTime)
}
