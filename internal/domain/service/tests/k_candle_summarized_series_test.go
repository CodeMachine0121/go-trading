package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var summarizedSeriesNow = time.Date(2026, 10, 18, 10, 20, 0, 0, time.UTC)

func octoberDay(day int) time.Time {
	return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC)
}

type summarizedContractSeriesUnderTest struct {
	service              *service.KCandleContractService
	kCandleRepository    *mocks.MockIKCandleContractRepository
	summaryDayRepository *mocks.MockIKCandleContractSummaryDayRepository
}

func newSummarizedContractSeriesUnderTest(t *testing.T) summarizedContractSeriesUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	summaryDayRepository := mocks.NewMockIKCandleContractSummaryDayRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(summarizedSeriesNow).AnyTimes()

	return summarizedContractSeriesUnderTest{
		service: service.NewKCandleContractService(kCandleRepository, summaryDayRepository, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}), 1000),
		kCandleRepository:    kCandleRepository,
		summaryDayRepository: summaryDayRepository,
	}
}

func (underTest summarizedContractSeriesUnderTest) series(
	t *testing.T, interval string, startTime time.Time, endTime time.Time,
) (dto.KCandleContractSeriesDto, error) {
	return underTest.service.GetKCandleContractSeries(t.Context(), dto.KCandleSeriesQueryDto{
		Symbol: "BTCUSDT", StartTime: startTime, EndTime: endTime, Interval: interval,
	})
}

// expectMinutes answers one minute read of exactly this stretch with these minutes.
func (underTest summarizedContractSeriesUnderTest) expectMinutes(
	startTime time.Time, endTime time.Time, expectedLimit int, minutes []entities.KCandleContract, readError error,
) *gomock.Call {
	return underTest.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), stretchOf(startTime, endTime), expectedLimit).
		Return(minutes, readError)
}

type stretchMatcher struct{ startTime, endTime time.Time }

func stretchOf(startTime time.Time, endTime time.Time) gomock.Matcher {
	return stretchMatcher{startTime: startTime, endTime: endTime}
}

func (matcher stretchMatcher) Matches(candidate any) bool {
	query, isQuery := candidate.(domains.KCandleQueryDomain)

	return isQuery && query.StartTime().Equal(matcher.startTime) && query.EndTime().Equal(matcher.endTime)
}

func (matcher stretchMatcher) String() string {
	return "stretch " + matcher.startTime.String() + " – " + matcher.endTime.String()
}

func contractSummaryAt(interval vo.AggregationIntervalVo, openTime time.Time, openPrice string, closePrice string, volume string) entities.KCandleContractSummary {
	return entities.KCandleContractSummary{
		Symbol: "BTCUSDT", Interval: string(interval), OpenTime: openTime,
		Open: decimal.RequireFromString(openPrice), High: decimal.RequireFromString("200"),
		Low: decimal.RequireFromString("50"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString(volume), TradeCount: 1,
		MarkClose: decimal.RequireFromString(closePrice),
	}
}

func summarizedContractDay(day time.Time, summaries ...entities.KCandleContractSummary) entities.KCandleContractSummaryDay {
	return entities.KCandleContractSummaryDay{
		Symbol: "BTCUSDT", Day: day, Revision: 1, Summarized: true, Summaries: summaries,
	}
}

func contractMinuteAt(openTime time.Time, closePrice string, volume string) entities.KCandleContract {
	return entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: openTime,
		Open: decimal.RequireFromString(closePrice), High: decimal.RequireFromString(closePrice),
		Low: decimal.RequireFromString(closePrice), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString(volume), TradeCount: 1,
		MarkClose: decimal.RequireFromString(closePrice),
	}
}

const (
	minutesInADay        = 1440
	lastNanosecondOfADay = 24*time.Hour - time.Nanosecond
)

func TestDailySeriesOfSummarizedDaysIsTakenFromTheSummaries(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(10), octoberDay(13)).
		Return([]entities.KCandleContractSummaryDay{
			summarizedContractDay(octoberDay(10), contractSummaryAt(vo.AggregationIntervalOneDay, octoberDay(10), "100", "101", "10")),
			summarizedContractDay(octoberDay(11), contractSummaryAt(vo.AggregationIntervalOneDay, octoberDay(11), "101", "102", "11")),
			summarizedContractDay(octoberDay(12), contractSummaryAt(vo.AggregationIntervalOneDay, octoberDay(12), "102", "103", "12")),
		}, nil)
	underTest.expectMinutes(octoberDay(13), octoberDay(13), 1, nil, nil)

	series, seriesError := underTest.series(t, "1d", octoberDay(10), octoberDay(13))
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 3)
	for index, expectedClose := range []string{"101", "102", "103"} {
		assert.Equal(t, octoberDay(10+index), series.KCandles[index].OpenTime)
		assert.True(t, series.KCandles[index].Close.Equal(decimal.RequireFromString(expectedClose)))
	}
	assert.True(t, series.KCandles[2].Volume.Equal(decimal.RequireFromString("12")))
}

func TestFourHourSeriesIsMergedFromHourlySummaries(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	hourlySummaries := make([]entities.KCandleContractSummary, 0, 24)
	for hour := range 24 {
		price := decimal.NewFromInt(int64(100 + hour)).String()
		hourlySummaries = append(hourlySummaries, contractSummaryAt(vo.AggregationIntervalOneHour,
			octoberDay(10).Add(time.Duration(hour)*time.Hour), price, price, decimal.NewFromInt(int64(hour+1)).String()))
	}
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneHour, octoberDay(10), octoberDay(11)).
		Return([]entities.KCandleContractSummaryDay{summarizedContractDay(octoberDay(10), hourlySummaries...)}, nil)
	underTest.expectMinutes(octoberDay(11), octoberDay(11), 1, nil, nil)

	series, seriesError := underTest.series(t, "4h", octoberDay(10), octoberDay(11))
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 6)
	assert.True(t, series.KCandles[0].Open.Equal(decimal.RequireFromString("100")))
	assert.True(t, series.KCandles[0].Close.Equal(decimal.RequireFromString("103")))
	assert.True(t, series.KCandles[0].Volume.Equal(decimal.RequireFromString("10")))
	assert.Equal(t, int64(4), series.KCandles[0].TradeCount)
	assert.Equal(t, octoberDay(10).Add(20*time.Hour), series.KCandles[5].OpenTime)
}

func TestTodaysDailyCandleIsMergedFromTodaysMinutes(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(17), octoberDay(18)).
		Return([]entities.KCandleContractSummaryDay{
			summarizedContractDay(octoberDay(17), contractSummaryAt(vo.AggregationIntervalOneDay, octoberDay(17), "100", "101", "10")),
		}, nil)
	underTest.expectMinutes(octoberDay(18), summarizedSeriesNow, 621, []entities.KCandleContract{
		contractMinuteAt(octoberDay(18).Add(10*time.Hour+19*time.Minute), "108", "2"),
		contractMinuteAt(octoberDay(18).Add(time.Minute), "104", "1"),
	}, nil)

	series, seriesError := underTest.series(t, "1d", octoberDay(17), summarizedSeriesNow)
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 2)
	assert.True(t, series.KCandles[0].Close.Equal(decimal.RequireFromString("101")))
	assert.Equal(t, octoberDay(18), series.KCandles[1].OpenTime)
	assert.True(t, series.KCandles[1].Open.Equal(decimal.RequireFromString("104")))
	assert.True(t, series.KCandles[1].Close.Equal(decimal.RequireFromString("108")))
	assert.True(t, series.KCandles[1].Volume.Equal(decimal.RequireFromString("3")))
}

func TestTodaysHourlyCandlesAreMergedFromTodaysMinutes(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneHour, octoberDay(17), octoberDay(18)).
		Return([]entities.KCandleContractSummaryDay{summarizedContractDay(octoberDay(17),
			contractSummaryAt(vo.AggregationIntervalOneHour, octoberDay(17).Add(23*time.Hour), "100", "101", "10"))}, nil)
	underTest.expectMinutes(octoberDay(18), summarizedSeriesNow, 621, []entities.KCandleContract{
		contractMinuteAt(octoberDay(18), "104", "1"),
		contractMinuteAt(octoberDay(18).Add(10*time.Hour+5*time.Minute), "108", "2"),
	}, nil)

	series, seriesError := underTest.series(t, "1h", octoberDay(17), summarizedSeriesNow)
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 3)
	assert.Equal(t, octoberDay(17).Add(23*time.Hour), series.KCandles[0].OpenTime)
	assert.Equal(t, octoberDay(18), series.KCandles[1].OpenTime)
	assert.Equal(t, octoberDay(18).Add(10*time.Hour), series.KCandles[2].OpenTime)
	assert.True(t, series.KCandles[2].Close.Equal(decimal.RequireFromString("108")))
}

func TestFineSeriesStillMergesEveryMinute(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	// The unchanged source limit: four round-the-clock buckets plus the spare one, fifteen minutes each.
	underTest.expectMinutes(octoberDay(13), octoberDay(13).Add(time.Hour), 75, []entities.KCandleContract{
		contractMinuteAt(octoberDay(13).Add(time.Minute), "104", "1"),
	}, nil)

	series, seriesError := underTest.series(t, "15m", octoberDay(13), octoberDay(13).Add(time.Hour))
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 1)
	assert.True(t, series.KCandles[0].Close.Equal(decimal.RequireFromString("104")))
}

func TestMissingDaysAreMergedFromTheirMinutesAndStored(t *testing.T) {
	testCases := []struct {
		name             string
		missingDayState  []entities.KCandleContractSummaryDay
		expectedRevision int64
		isStored         bool
	}{
		{"a day never summarized", nil, 0, true},
		{"a day voided since it was summarized", []entities.KCandleContractSummaryDay{
			{Symbol: "BTCUSDT", Day: octoberDay(11), Revision: 3},
		}, 3, true},
		{"a day changed while it was merged is still answered", nil, 0, false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSummarizedContractSeriesUnderTest(t)
			underTest.summaryDayRepository.EXPECT().
				FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(10), octoberDay(12)).
				Return(append([]entities.KCandleContractSummaryDay{
					summarizedContractDay(octoberDay(10), contractSummaryAt(vo.AggregationIntervalOneDay, octoberDay(10), "100", "101", "10")),
				}, testCase.missingDayState...), nil)
			underTest.expectMinutes(octoberDay(11), octoberDay(11).Add(lastNanosecondOfADay), minutesInADay,
				[]entities.KCandleContract{
					contractMinuteAt(octoberDay(11).Add(9*time.Hour), "105", "2"),
					contractMinuteAt(octoberDay(11).Add(9*time.Hour+30*time.Minute), "107", "3"),
				}, nil)
			underTest.summaryDayRepository.EXPECT().
				SaveSummarized(gomock.Any(), gomock.Cond(func(summaryDay entities.KCandleContractSummaryDay) bool {
					return summaryDay.Day.Equal(octoberDay(11)) && summaryDay.Revision == testCase.expectedRevision &&
						summaryDay.Summarized && len(summaryDay.Summaries) == 2
				})).
				Return(testCase.isStored, nil)
			underTest.expectMinutes(octoberDay(12), octoberDay(12), 1, nil, nil)

			series, seriesError := underTest.series(t, "1d", octoberDay(10), octoberDay(12))
			require.NoError(t, seriesError)

			require.Len(t, series.KCandles, 2)
			assert.True(t, series.KCandles[0].Close.Equal(decimal.RequireFromString("101")))
			assert.Equal(t, octoberDay(11), series.KCandles[1].OpenTime)
			assert.True(t, series.KCandles[1].Open.Equal(decimal.RequireFromString("105")))
			assert.True(t, series.KCandles[1].Close.Equal(decimal.RequireFromString("107")))
			assert.True(t, series.KCandles[1].Volume.Equal(decimal.RequireFromString("5")))
		})
	}
}

func TestDayWithoutMinutesIsStoredAsEmptyAndHasNoCandle(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(11), octoberDay(12)).
		Return(nil, nil)
	underTest.expectMinutes(octoberDay(11), octoberDay(11).Add(lastNanosecondOfADay), minutesInADay, nil, nil)
	underTest.summaryDayRepository.EXPECT().
		SaveSummarized(gomock.Any(), gomock.Cond(func(summaryDay entities.KCandleContractSummaryDay) bool {
			return summaryDay.Summarized && len(summaryDay.Summaries) == 0
		})).
		Return(true, nil)
	underTest.expectMinutes(octoberDay(12), octoberDay(12), 1, nil, nil)

	series, seriesError := underTest.series(t, "1d", octoberDay(11), octoberDay(12))
	require.NoError(t, seriesError)

	assert.Empty(t, series.KCandles)
}

func TestFillingStopsAtTheFirstDayThatCannotBeReadKeepingTheDaysBefore(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	readFailure := errors.New("database unreachable")
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(10), octoberDay(13)).
		Return(nil, nil)
	gomock.InOrder(
		underTest.expectMinutes(octoberDay(10), octoberDay(10).Add(lastNanosecondOfADay), minutesInADay, nil, nil),
		underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(),
			gomock.Cond(func(summaryDay entities.KCandleContractSummaryDay) bool {
				return summaryDay.Day.Equal(octoberDay(10))
			})).Return(true, nil),
		underTest.expectMinutes(octoberDay(11), octoberDay(11).Add(lastNanosecondOfADay), minutesInADay, nil, readFailure),
	)

	_, seriesError := underTest.series(t, "1d", octoberDay(10), octoberDay(13))

	assert.ErrorIs(t, seriesError, readFailure)
}

type summarizedSpotSeriesUnderTest struct {
	service              *service.KCandleService
	kCandleRepository    *mocks.MockIKCandleRepository
	summaryDayRepository *mocks.MockIKCandleSummaryDayRepository
}

func newSummarizedSpotSeriesUnderTest(t *testing.T) summarizedSpotSeriesUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	summaryDayRepository := mocks.NewMockIKCandleSummaryDayRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.TradingSymbol{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(summarizedSeriesNow).AnyTimes()

	return summarizedSpotSeriesUnderTest{
		service: service.NewKCandleService(kCandleRepository, summaryDayRepository, tradingSymbolRepository, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}), 1000),
		kCandleRepository:    kCandleRepository,
		summaryDayRepository: summaryDayRepository,
	}
}

func TestSpotDailySeriesKeepsFiguresNoMinuteReportedUnreported(t *testing.T) {
	underTest := newSummarizedSpotSeriesUnderTest(t)
	minuteWithoutQuoteVolume := entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: octoberDay(11).Add(time.Hour),
		Open: decimal.RequireFromString("100"), High: decimal.RequireFromString("100"),
		Low: decimal.RequireFromString("100"), Close: decimal.RequireFromString("100"),
		Volume: decimal.RequireFromString("1"),
	}
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(11), octoberDay(12)).
		Return(nil, nil)
	underTest.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), stretchOf(octoberDay(11), octoberDay(11).Add(lastNanosecondOfADay)), minutesInADay).
		Return([]entities.KCandle{minuteWithoutQuoteVolume}, nil)
	underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), gomock.Any()).Return(true, nil)
	underTest.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), stretchOf(octoberDay(12), octoberDay(12)), 1).Return(nil, nil)

	series, seriesError := underTest.service.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
		Symbol: "BTCUSDT", StartTime: octoberDay(11), EndTime: octoberDay(12), Interval: "1d",
	})
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 1)
	assert.False(t, series.KCandles[0].QuoteVolume.Valid)
	assert.True(t, series.KCandles[0].Volume.Equal(decimal.RequireFromString("1")))
}

func TestCoarseSeriesReportsWhicheverReadFails(t *testing.T) {
	failure := errors.New("storage unreachable")
	testCases := []struct {
		name    string
		arrange func(underTest summarizedContractSeriesUnderTest)
	}{
		{"reading the day states", func(underTest summarizedContractSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, failure)
		}},
		{"storing a merged day", func(underTest summarizedContractSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.expectMinutes(octoberDay(11), octoberDay(11).Add(lastNanosecondOfADay), minutesInADay, nil, nil)
			underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), gomock.Any()).Return(false, failure)
		}},
		{"reading the partial end", func(underTest summarizedContractSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]entities.KCandleContractSummaryDay{{Day: octoberDay(11), Summarized: true}}, nil)
			underTest.expectMinutes(octoberDay(12), octoberDay(12), 1, nil, failure)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSummarizedContractSeriesUnderTest(t)
			testCase.arrange(underTest)

			_, seriesError := underTest.series(t, "1d", octoberDay(11), octoberDay(12))

			assert.ErrorIs(t, seriesError, failure)
		})
	}
}

func TestSpotCoarseSeriesReportsWhicheverReadFails(t *testing.T) {
	failure := errors.New("storage unreachable")
	testCases := []struct {
		name    string
		arrange func(underTest summarizedSpotSeriesUnderTest)
	}{
		{"reading the day states", func(underTest summarizedSpotSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, failure)
		}},
		{"reading a missing day", func(underTest summarizedSpotSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), minutesInADay).Return(nil, failure)
		}},
		{"storing a merged day", func(underTest summarizedSpotSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), minutesInADay).Return(nil, nil)
			underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), gomock.Any()).Return(false, failure)
		}},
		{"reading the partial end", func(underTest summarizedSpotSeriesUnderTest) {
			underTest.summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]entities.KCandleSummaryDay{{Day: octoberDay(11), Revision: 2, Summarized: true,
					Summaries: []entities.KCandleSummary{{Symbol: "BTCUSDT", Interval: "1d", OpenTime: octoberDay(11)}}}}, nil)
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), stretchOf(octoberDay(12), octoberDay(12)), 1).
				Return(nil, failure)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSummarizedSpotSeriesUnderTest(t)
			testCase.arrange(underTest)

			_, seriesError := underTest.service.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
				Symbol: "BTCUSDT", StartTime: octoberDay(11), EndTime: octoberDay(12), Interval: "1d",
			})

			assert.ErrorIs(t, seriesError, failure)
		})
	}
}

func TestNineHundredSummarizedDaysAnswerEveryFigureOfEachDaysSummary(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	firstDay := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	lastDayEnd := firstDay.Add(900 * domains.KCandleSummaryDayLength)
	summaryDays := make([]entities.KCandleContractSummaryDay, 0, 900)
	for dayIndex := range 900 {
		day := firstDay.Add(time.Duration(dayIndex) * domains.KCandleSummaryDayLength)
		summary := entities.KCandleContractSummary{
			Symbol: "BTCUSDT", Interval: string(vo.AggregationIntervalOneDay), OpenTime: day,
			Open: decimal.NewFromInt(int64(1000 + dayIndex)), High: decimal.NewFromInt(int64(2000 + dayIndex)),
			Low: decimal.NewFromInt(int64(500 + dayIndex)), Close: decimal.NewFromInt(int64(1001 + dayIndex)),
			Volume: decimal.NewFromInt(int64(dayIndex + 1)), QuoteVolume: decimal.NewFromInt(int64(dayIndex + 2)),
			TakerBuyBaseVolume: decimal.NewFromInt(int64(dayIndex + 3)), TakerBuyQuoteVolume: decimal.NewFromInt(int64(dayIndex + 4)),
			TradeCount: int64(dayIndex + 5),
			MarkOpen:   decimal.NewFromInt(int64(1100 + dayIndex)), MarkHigh: decimal.NewFromInt(int64(2100 + dayIndex)),
			MarkLow: decimal.NewFromInt(int64(600 + dayIndex)), MarkClose: decimal.NewFromInt(int64(1101 + dayIndex)),
			IndexOpen:         decimal.NewNullDecimal(decimal.NewFromInt(int64(1200 + dayIndex))),
			IndexHigh:         decimal.NewNullDecimal(decimal.NewFromInt(int64(2200 + dayIndex))),
			IndexLow:          decimal.NewNullDecimal(decimal.NewFromInt(int64(700 + dayIndex))),
			IndexClose:        decimal.NewNullDecimal(decimal.NewFromInt(int64(1201 + dayIndex))),
			PremiumIndexOpen:  decimal.NewNullDecimal(decimal.RequireFromString("0.1")),
			PremiumIndexHigh:  decimal.NewNullDecimal(decimal.RequireFromString("0.3")),
			PremiumIndexLow:   decimal.NewNullDecimal(decimal.RequireFromString("-0.1")),
			PremiumIndexClose: decimal.NewNullDecimal(decimal.RequireFromString("0.2")),
		}
		summaryDays = append(summaryDays, summarizedContractDay(day, summary))
	}
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, firstDay, lastDayEnd).
		Return(summaryDays, nil)
	underTest.expectMinutes(lastDayEnd, lastDayEnd, 1, nil, nil)

	series, seriesError := underTest.series(t, "1d", firstDay, lastDayEnd)
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 900)
	for dayIndex, summaryDay := range summaryDays {
		// A day merged from its own single summary must read exactly as that summary.
		assert.Equal(t, summaryDay.Summaries[0].ToKCandleContract().ToDto(), series.KCandles[dayIndex])
	}
}

func TestFourHourBucketWithOneHourMissingThePremiumHasNoPremium(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	hourlySummaries := make([]entities.KCandleContractSummary, 0, 24)
	for hour := range 24 {
		price := decimal.NewFromInt(int64(100 + hour)).String()
		summary := contractSummaryAt(vo.AggregationIntervalOneHour,
			octoberDay(10).Add(time.Duration(hour)*time.Hour), price, price, "1")
		if hour != 1 {
			summary.PremiumIndexOpen = decimal.NewNullDecimal(decimal.RequireFromString("0.1"))
			summary.PremiumIndexHigh = decimal.NewNullDecimal(decimal.RequireFromString("0.2"))
			summary.PremiumIndexLow = decimal.NewNullDecimal(decimal.RequireFromString("0.0"))
			summary.PremiumIndexClose = decimal.NewNullDecimal(decimal.RequireFromString("0.1"))
		}
		hourlySummaries = append(hourlySummaries, summary)
	}
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneHour, octoberDay(10), octoberDay(11)).
		Return([]entities.KCandleContractSummaryDay{summarizedContractDay(octoberDay(10), hourlySummaries...)}, nil)
	underTest.expectMinutes(octoberDay(11), octoberDay(11), 1, nil, nil)

	series, seriesError := underTest.series(t, "4h", octoberDay(10), octoberDay(11))
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 6)
	assert.False(t, series.KCandles[0].PremiumIndexOpen.Valid, "the 01:00 hour has no premium index")
	assert.True(t, series.KCandles[0].MarkClose.Equal(decimal.RequireFromString("103")))
	assert.True(t, series.KCandles[0].Volume.Equal(decimal.RequireFromString("4")))
	assert.True(t, series.KCandles[1].PremiumIndexClose.Valid, "04:00–08:00 has every hour's premium index")
}

func TestWeekWithTwoDaysWithoutMinutesHasFiveDailyCandles(t *testing.T) {
	underTest := newSummarizedContractSeriesUnderTest(t)
	underTest.summaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay, octoberDay(10), octoberDay(17)).
		Return(nil, nil)
	for day := 10; day < 17; day++ {
		minutes := []entities.KCandleContract{contractMinuteAt(octoberDay(day).Add(time.Hour), "100", "1")}
		if day == 11 || day == 12 {
			minutes = nil
		}
		underTest.expectMinutes(octoberDay(day), octoberDay(day).Add(lastNanosecondOfADay), minutesInADay, minutes, nil)
	}
	storedEmptyDays := 0
	underTest.summaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, summaryDay entities.KCandleContractSummaryDay) (bool, error) {
			if len(summaryDay.Summaries) == 0 {
				storedEmptyDays++
			}

			return true, nil
		}).Times(7)
	underTest.expectMinutes(octoberDay(17), octoberDay(17), 1, nil, nil)

	series, seriesError := underTest.series(t, "1d", octoberDay(10), octoberDay(17))
	require.NoError(t, seriesError)

	require.Len(t, series.KCandles, 5)
	for _, kCandle := range series.KCandles {
		assert.NotEqual(t, octoberDay(11), kCandle.OpenTime)
		assert.NotEqual(t, octoberDay(12), kCandle.OpenTime)
	}
	assert.Equal(t, 2, storedEmptyDays, "the two empty days hold no summary candle")
}
