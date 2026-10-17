package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleSummaryDayLength is the unit summaries are filled, invalidated and versioned in: one UTC day, the coarsest bucket, so every hour belongs to exactly one day.
const KCandleSummaryDayLength = 24 * time.Hour

// KCandleSummaryPlanDomain splits a coarse series request into finished days' summaries and live-merged minutes at both ends, so a request holds about a day of minutes at most.
type KCandleSummaryPlanDomain struct {
	rangeQuery      KCandleQueryDomain
	summaryInterval AggregationIntervalDomain
	usesSummaries   bool
	summarizedStart time.Time
	summarizedEnd   time.Time
}

// NewKCandleSummaryPlanDomain keeps today out of the summarized stretch, since only finished days are summarized.
func NewKCandleSummaryPlanDomain(
	seriesQueryDomain KCandleSeriesQueryDomain, now time.Time,
) KCandleSummaryPlanDomain {
	rangeQuery := seriesQueryDomain.rangeQuery
	interval := seriesQueryDomain.interval

	summarizedStart := interval.BucketStart(rangeQuery.startTime)
	if summarizedStart.Before(rangeQuery.startTime) {
		summarizedStart = summarizedStart.Add(interval.duration)
	}

	summarizedEnd := interval.BucketStart(rangeQuery.endTime)
	if todayStart := bucketStartOf(now, KCandleSummaryDayLength); todayStart.Before(summarizedEnd) {
		summarizedEnd = todayStart
	}

	summaryInterval, hasSummaries := interval.summaryInterval()

	return KCandleSummaryPlanDomain{
		rangeQuery:      rangeQuery,
		summaryInterval: summaryInterval,
		usesSummaries:   hasSummaries && summarizedStart.Before(summarizedEnd),
		summarizedStart: summarizedStart,
		summarizedEnd:   summarizedEnd,
	}
}

// UsesSummaries is false for fine intervals and for requests too short to contain one whole finished bucket; those keep merging minutes as before.
func (planDomain KCandleSummaryPlanDomain) UsesSummaries() bool {
	return planDomain.usesSummaries
}

func (planDomain KCandleSummaryPlanDomain) Symbol() string {
	return planDomain.rangeQuery.symbol
}

func (planDomain KCandleSummaryPlanDomain) SummaryInterval() vo.AggregationIntervalVo {
	return planDomain.summaryInterval.value
}

func (planDomain KCandleSummaryPlanDomain) SummarizedStart() time.Time {
	return planDomain.summarizedStart
}

func (planDomain KCandleSummaryPlanDomain) SummarizedEnd() time.Time {
	return planDomain.summarizedEnd
}

// MinuteStretches are the partial bucket before the summarized stretch, if any, and everything from its end on, which is at most one bucket or today.
func (planDomain KCandleSummaryPlanDomain) MinuteStretches() []KCandleQueryDomain {
	stretches := make([]KCandleQueryDomain, 0, 2)
	if planDomain.rangeQuery.startTime.Before(planDomain.summarizedStart) {
		stretches = append(stretches, KCandleQueryDomain{
			symbol:    planDomain.rangeQuery.symbol,
			startTime: planDomain.rangeQuery.startTime,
			endTime:   planDomain.summarizedStart.Add(-time.Nanosecond),
		})
	}

	return append(stretches, KCandleQueryDomain{
		symbol:    planDomain.rangeQuery.symbol,
		startTime: planDomain.summarizedEnd,
		endTime:   planDomain.rangeQuery.endTime,
	})
}

// DaysToSummarize lists, earliest first, the days the summarized stretch touches that are not among the summarized ones.
func (planDomain KCandleSummaryPlanDomain) DaysToSummarize(summarizedDays []time.Time) []time.Time {
	isSummarized := make(map[time.Time]bool, len(summarizedDays))
	for _, summarizedDay := range summarizedDays {
		isSummarized[summarizedDay.UTC()] = true
	}

	daysToSummarize := make([]time.Time, 0)
	for day := bucketStartOf(planDomain.summarizedStart, KCandleSummaryDayLength); day.Before(planDomain.summarizedEnd); day = day.Add(KCandleSummaryDayLength) {
		if !isSummarized[day] {
			daysToSummarize = append(daysToSummarize, day)
		}
	}

	return daysToSummarize
}

// DayStretch is every minute of one day the plan summarizes, the unit a missing day is merged in.
func (planDomain KCandleSummaryPlanDomain) DayStretch(day time.Time) KCandleQueryDomain {
	return KCandleQueryDomain{
		symbol:    planDomain.rangeQuery.symbol,
		startTime: day,
		endTime:   day.Add(KCandleSummaryDayLength - time.Nanosecond),
	}
}

// IncludesSummary tells a freshly merged day's summaries that belong to this request from the rest of that day.
func (planDomain KCandleSummaryPlanDomain) IncludesSummary(
	interval vo.AggregationIntervalVo, openTime time.Time,
) bool {
	return interval == planDomain.summaryInterval.value &&
		!openTime.Before(planDomain.summarizedStart) && openTime.Before(planDomain.summarizedEnd)
}
