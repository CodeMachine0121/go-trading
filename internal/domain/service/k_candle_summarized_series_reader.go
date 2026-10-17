package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleSummarizedSeriesReader gathers a coarse spot series' source candles: minutes for the partial ends and stored summaries for whole finished days, merging missing days one at a time so at most a day of minutes is held.
type kCandleSummarizedSeriesReader struct {
	kCandleRepository    domaininterface.IKCandleRepository
	summaryDayRepository domaininterface.IKCandleSummaryDayRepository
}

func newKCandleSummarizedSeriesReader(
	kCandleRepository domaininterface.IKCandleRepository,
	summaryDayRepository domaininterface.IKCandleSummaryDayRepository,
) kCandleSummarizedSeriesReader {
	return kCandleSummarizedSeriesReader{
		kCandleRepository: kCandleRepository, summaryDayRepository: summaryDayRepository,
	}
}

// read returns candles in the shape of stored minutes, so the series merges summaries exactly as it merges minutes.
func (reader kCandleSummarizedSeriesReader) read(
	executionContext context.Context, planDomain domains.KCandleSummaryPlanDomain,
) ([]entities.KCandle, error) {
	summaries, summariesError := reader.summaries(executionContext, planDomain)
	if summariesError != nil {
		return nil, summariesError
	}

	sourceCandles := make([]entities.KCandle, 0, len(summaries))
	for _, summary := range summaries {
		sourceCandles = append(sourceCandles, summary.ToKCandle())
	}
	for _, minuteStretch := range planDomain.MinuteStretches() {
		minutes, findError := reader.kCandleRepository.FindInRange(
			executionContext, minuteStretch, minuteStretch.MinuteCapacity())
		if findError != nil {
			return nil, findError
		}
		sourceCandles = append(sourceCandles, minutes...)
	}

	return sourceCandles, nil
}

// summaries uses a day merged while it changed for this answer but leaves storing it to the next request, which will see the change.
func (reader kCandleSummarizedSeriesReader) summaries(
	executionContext context.Context, planDomain domains.KCandleSummaryPlanDomain,
) ([]entities.KCandleSummary, error) {
	summaryDays, findError := reader.summaryDayRepository.FindDaysInRange(executionContext,
		planDomain.Symbol(), planDomain.SummaryInterval(), planDomain.SummarizedStart(), planDomain.SummarizedEnd())
	if findError != nil {
		return nil, findError
	}

	summaries := make([]entities.KCandleSummary, 0)
	summarizedDays := make([]time.Time, 0, len(summaryDays))
	revisionByDay := make(map[time.Time]int64, len(summaryDays))
	for _, summaryDay := range summaryDays {
		revisionByDay[summaryDay.Day] = summaryDay.Revision
		if summaryDay.Summarized {
			summarizedDays = append(summarizedDays, summaryDay.Day)
			summaries = append(summaries, summaryDay.Summaries...)
		}
	}

	for _, day := range planDomain.DaysToSummarize(summarizedDays) {
		dayQuery, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
			Symbol:    planDomain.Symbol(),
			StartTime: day,
			EndTime:   day.Add(domains.KCandleSummaryDayLength - time.Nanosecond),
		})
		if queryError != nil {
			return nil, queryError
		}
		minutes, minutesError := reader.kCandleRepository.FindInRange(
			executionContext, dayQuery, dayQuery.MinuteCapacity())
		if minutesError != nil {
			return nil, minutesError
		}

		mergedDay := domains.NewKCandleSummaryDayDomain(
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
