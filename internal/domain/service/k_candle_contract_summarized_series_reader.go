package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

type kCandleContractSummarizedSeriesReader struct {
	kCandleContractRepository domaininterface.IKCandleContractRepository
	summaryDayRepository      domaininterface.IKCandleContractSummaryDayRepository
}

func newKCandleContractSummarizedSeriesReader(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	summaryDayRepository domaininterface.IKCandleContractSummaryDayRepository,
) kCandleContractSummarizedSeriesReader {
	return kCandleContractSummarizedSeriesReader{
		kCandleContractRepository: kCandleContractRepository, summaryDayRepository: summaryDayRepository,
	}
}

func (reader kCandleContractSummarizedSeriesReader) read(
	executionContext context.Context, seriesQueryDomain domains.KCandleSeriesQueryDomain, now time.Time,
) ([]entities.KCandleContract, error) {
	planDomain := domains.NewKCandleSummaryPlanDomain(seriesQueryDomain, now)
	if !planDomain.UsesSummaries() {
		return reader.kCandleContractRepository.FindInRange(
			executionContext, seriesQueryDomain.RangeQuery(), seriesQueryDomain.SourceCandleLimit())
	}

	summaries, summariesError := reader.summaries(executionContext, planDomain)
	if summariesError != nil {
		return nil, summariesError
	}

	sourceCandles := make([]entities.KCandleContract, 0, len(summaries))
	for _, summary := range summaries {
		sourceCandles = append(sourceCandles, summary.ToKCandleContract())
	}
	for _, minuteStretch := range planDomain.MinuteStretches() {
		minutes, findError := reader.kCandleContractRepository.FindInRange(
			executionContext, minuteStretch, minuteStretch.MinuteCapacity())
		if findError != nil {
			return nil, findError
		}
		sourceCandles = append(sourceCandles, minutes...)
	}

	return sourceCandles, nil
}

func (reader kCandleContractSummarizedSeriesReader) summaries(
	executionContext context.Context, planDomain domains.KCandleSummaryPlanDomain,
) ([]entities.KCandleContractSummary, error) {
	summaryDays, findError := reader.summaryDayRepository.FindDaysInRange(executionContext,
		planDomain.Symbol(), planDomain.SummaryInterval(), planDomain.SummarizedStart(), planDomain.SummarizedEnd())
	if findError != nil {
		return nil, findError
	}

	summaries := make([]entities.KCandleContractSummary, 0)
	summarizedDays := make([]time.Time, 0, len(summaryDays))
	revisionByDay := make(map[time.Time]int64, len(summaryDays))
	for _, summaryDay := range summaryDays {
		revisionByDay[summaryDay.Day] = summaryDay.Revision
		if summaryDay.Summarized {
			summarizedDays = append(summarizedDays, summaryDay.Day)
			summaries = append(summaries, summaryDay.Summaries...)
		}
	}

	daysToSummarize := planDomain.DaysToSummarize(summarizedDays)
	if len(daysToSummarize) > 0 {
		probeStretch := planDomain.StretchFromDay(daysToSummarize[0])
		anyMinute, probeError := reader.kCandleContractRepository.FindInRange(executionContext, probeStretch, 1)
		if probeError != nil {
			return nil, probeError
		}
		// Nothing stored there yet (a mistyped symbol, or history not synced): leave the days for when minutes exist.
		if len(anyMinute) == 0 {
			daysToSummarize = nil
		} else {
			daysToSummarize = planDomain.DaysFromFirstMinute(daysToSummarize, anyMinute[0].OpenTime)
		}
	}

	for _, day := range daysToSummarize {
		dayQuery := planDomain.DayStretch(day)
		minutes, minutesError := reader.kCandleContractRepository.FindInRange(
			executionContext, dayQuery, dayQuery.MinuteCapacity())
		if minutesError != nil {
			return nil, minutesError
		}

		mergedDay := domains.NewKCandleContractSummaryDayDomain(
			planDomain.Symbol(), day, revisionByDay[day], minutes).ToEntity()
		if _, saveError := reader.summaryDayRepository.SaveSummarized(executionContext, mergedDay); saveError != nil {
			return nil, saveError
		}

		for _, summary := range mergedDay.Summaries {
			if planDomain.IncludesSummary(vo.AggregationIntervalVo(summary.Interval), summary.OpenTime) {
				summaries = append(summaries, summary)
			}
		}
	}

	return summaries, nil
}

// readLatestBuckets gathers the source candles of the latest finished buckets an indicator calculation needs, reading summaries stretch by stretch and jumping over stretches with no candles until enough buckets are found or history runs out.
func (reader kCandleContractSummarizedSeriesReader) readLatestBuckets(
	executionContext context.Context, reachDomain domains.IndicatorSourceReachDomain, now time.Time,
) ([]entities.KCandleContract, error) {
	if !reachDomain.ReadsSummaries() {
		return reader.kCandleContractRepository.FindLatestBefore(
			executionContext, reachDomain.Symbol(), reachDomain.Cutoff(), reachDomain.SourceCandleLimit())
	}

	sourceCandles := make([]entities.KCandleContract, 0)
	for {
		stretchCandles, readError := reader.read(executionContext, reachDomain.SeriesQuery(), now)
		if readError != nil {
			return nil, readError
		}
		sourceCandles = append(sourceCandles, stretchCandles...)

		reachDomain = reachDomain.CountingKCandleContracts(stretchCandles)
		if reachDomain.IsCovered() {
			return sourceCandles, nil
		}

		earlierCandles, probeError := reader.kCandleContractRepository.FindLatestBefore(
			executionContext, reachDomain.Symbol(), reachDomain.EarlierCutoff(), 1)
		if probeError != nil {
			return nil, probeError
		}
		if len(earlierCandles) == 0 {
			return sourceCandles, nil
		}

		reachDomain = reachDomain.ReachingBackTo(earlierCandles[0].OpenTime)
	}
}
