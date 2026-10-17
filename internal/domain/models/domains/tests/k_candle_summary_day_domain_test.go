package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var summaryDay = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

func summaryFigure(text string) decimal.Decimal {
	return decimal.RequireFromString(text)
}

func reportedSummaryFigure(text string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(text))
}

func spotMinute(hour int, minute int, open string, high string, low string, closePrice string, volume string) entities.KCandle {
	return entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: summaryDay.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute),
		Open: summaryFigure(open), High: summaryFigure(high), Low: summaryFigure(low), Close: summaryFigure(closePrice), Volume: summaryFigure(volume),
	}
}

func summaryOf(t *testing.T, summaryDayEntity entities.KCandleSummaryDay, interval string, openTime time.Time) entities.KCandleSummary {
	t.Helper()

	for _, summary := range summaryDayEntity.Summaries {
		if summary.Interval == interval && summary.OpenTime.Equal(openTime) {
			return summary
		}
	}
	require.Failf(t, "summary missing", "%s at %s", interval, openTime)

	return entities.KCandleSummary{}
}

func TestSpotDayMergesEachHourAndTheWholeDay(t *testing.T) {
	minutes := []entities.KCandle{
		spotMinute(14, 30, "103", "108", "101", "106", "2"),
		spotMinute(14, 5, "100", "105", "99", "103", "1"),
		spotMinute(15, 10, "106", "112", "104", "110", "4"),
	}

	summaryDayEntity := domains.NewKCandleSummaryDayDomain("BTCUSDT", summaryDay, 3, minutes).ToEntity()

	assert.True(t, summaryDayEntity.Summarized)
	assert.Equal(t, int64(3), summaryDayEntity.Revision)
	assert.Len(t, summaryDayEntity.Summaries, 3)
	fourteenHundred := summaryOf(t, summaryDayEntity, "1h", summaryDay.Add(14*time.Hour))
	assert.True(t, fourteenHundred.Open.Equal(summaryFigure("100")))
	assert.True(t, fourteenHundred.Close.Equal(summaryFigure("106")))
	assert.True(t, fourteenHundred.High.Equal(summaryFigure("108")))
	assert.True(t, fourteenHundred.Low.Equal(summaryFigure("99")))
	assert.True(t, fourteenHundred.Volume.Equal(summaryFigure("3")))
	wholeDay := summaryOf(t, summaryDayEntity, "1d", summaryDay)
	assert.True(t, wholeDay.Open.Equal(summaryFigure("100")))
	assert.True(t, wholeDay.Close.Equal(summaryFigure("110")))
	assert.True(t, wholeDay.High.Equal(summaryFigure("112")))
	assert.True(t, wholeDay.Low.Equal(summaryFigure("99")))
	assert.True(t, wholeDay.Volume.Equal(summaryFigure("7")))
}

func TestDayWithoutMinutesIsSummarizedWithNoCandles(t *testing.T) {
	summaryDayEntity := domains.NewKCandleSummaryDayDomain("2330", summaryDay, 0, nil).ToEntity()

	assert.True(t, summaryDayEntity.Summarized)
	assert.Empty(t, summaryDayEntity.Summaries)
}

func TestSpotDayKeepsFiguresTheMarketDoesNotReportUnreported(t *testing.T) {
	testCases := []struct {
		name                string
		firstQuoteVolume    decimal.NullDecimal
		secondQuoteVolume   decimal.NullDecimal
		expectedQuoteVolume decimal.NullDecimal
	}{
		{"no minute reports it", decimal.NullDecimal{}, decimal.NullDecimal{}, decimal.NullDecimal{}},
		{"only some minutes report it", decimal.NullDecimal{}, reportedSummaryFigure("5"), reportedSummaryFigure("5")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			first := spotMinute(1, 0, "100", "100", "100", "100", "1")
			first.QuoteVolume = testCase.firstQuoteVolume
			second := spotMinute(1, 1, "100", "100", "100", "100", "1")
			second.QuoteVolume = testCase.secondQuoteVolume

			summaryDayEntity := domains.NewKCandleSummaryDayDomain(
				"2330", summaryDay, 0, []entities.KCandle{first, second}).ToEntity()

			for _, interval := range []string{"1h", "1d"} {
				openTime := summaryDay
				if interval == "1h" {
					openTime = summaryDay.Add(time.Hour)
				}
				quoteVolume := summaryOf(t, summaryDayEntity, interval, openTime).QuoteVolume
				assert.Equal(t, testCase.expectedQuoteVolume.Valid, quoteVolume.Valid, interval)
				if testCase.expectedQuoteVolume.Valid {
					assert.True(t, testCase.expectedQuoteVolume.Decimal.Equal(quoteVolume.Decimal), interval)
				}
			}
		})
	}
}

func contractMinute(hour int, minute int, withIndex bool) entities.KCandleContract {
	kCandle := entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: summaryDay.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute),
		Open: summaryFigure("100"), High: summaryFigure("110"), Low: summaryFigure("90"), Close: summaryFigure("105"),
		Volume: summaryFigure("1"), TradeCount: 7,
		MarkOpen: summaryFigure("101"), MarkHigh: summaryFigure("111"), MarkLow: summaryFigure("91"), MarkClose: summaryFigure("106"),
		PremiumIndexOpen: reportedSummaryFigure("0.1"), PremiumIndexHigh: reportedSummaryFigure("0.2"),
		PremiumIndexLow: reportedSummaryFigure("0.0"), PremiumIndexClose: reportedSummaryFigure("0.1"),
	}
	if withIndex {
		kCandle.IndexOpen, kCandle.IndexHigh = reportedSummaryFigure("102"), reportedSummaryFigure("112")
		kCandle.IndexLow, kCandle.IndexClose = reportedSummaryFigure("92"), reportedSummaryFigure("107")
	}

	return kCandle
}

func TestContractDayWithOneMinuteMissingTheIndexHasNoIndexForThatDay(t *testing.T) {
	minutes := []entities.KCandleContract{contractMinute(3, 0, true), contractMinute(3, 1, false), contractMinute(4, 0, true)}

	summaryDayEntity := domains.NewKCandleContractSummaryDayDomain("BTCUSDT", summaryDay, 2, minutes).ToEntity()

	assert.True(t, summaryDayEntity.Summarized)
	assert.Equal(t, int64(2), summaryDayEntity.Revision)
	require.Len(t, summaryDayEntity.Summaries, 3)
	for _, summary := range summaryDayEntity.Summaries {
		switch {
		case summary.Interval == "1d":
			assert.False(t, summary.IndexOpen.Valid, "the day holds the minute without an index")
			assert.True(t, summary.PremiumIndexOpen.Valid)
			assert.True(t, summary.MarkClose.Equal(summaryFigure("106")))
			assert.Equal(t, int64(21), summary.TradeCount)
		case summary.OpenTime.Equal(summaryDay.Add(3 * time.Hour)):
			assert.False(t, summary.IndexOpen.Valid, "03:00 holds the minute without an index")
		default:
			assert.True(t, summary.IndexClose.Valid, "04:00 has every minute's index")
			assert.True(t, summary.IndexClose.Decimal.Equal(summaryFigure("107")))
		}
	}
}

func everyFigureContractMinute() entities.KCandleContract {
	return entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: summaryDay.Add(5 * time.Hour),
		Open: summaryFigure("1"), High: summaryFigure("2"), Low: summaryFigure("3"), Close: summaryFigure("4"),
		Volume: summaryFigure("5"), QuoteVolume: summaryFigure("6"),
		TakerBuyBaseVolume: summaryFigure("7"), TakerBuyQuoteVolume: summaryFigure("8"), TradeCount: 9,
		MarkOpen: summaryFigure("10"), MarkHigh: summaryFigure("11"), MarkLow: summaryFigure("12"), MarkClose: summaryFigure("13"),
		IndexOpen: reportedSummaryFigure("14"), IndexHigh: reportedSummaryFigure("15"),
		IndexLow: reportedSummaryFigure("16"), IndexClose: reportedSummaryFigure("17"),
		PremiumIndexOpen: reportedSummaryFigure("18"), PremiumIndexHigh: reportedSummaryFigure("19"),
		PremiumIndexLow: reportedSummaryFigure("20"), PremiumIndexClose: reportedSummaryFigure("21"),
	}
}

func TestSummaryOfASingleContractMinuteCarriesEveryFigure(t *testing.T) {
	minute := everyFigureContractMinute()

	summaryDayEntity := domains.NewKCandleContractSummaryDayDomain(
		"BTCUSDT", summaryDay, 0, []entities.KCandleContract{minute}).ToEntity()

	require.Len(t, summaryDayEntity.Summaries, 2)
	for _, summary := range summaryDayEntity.Summaries {
		roundTripped := summary.ToKCandleContract()
		roundTripped.OpenTime = minute.OpenTime
		assert.Equal(t, minute.ToDto(), roundTripped.ToDto(), summary.Interval)
	}
}

func TestSummaryOfASingleSpotMinuteCarriesEveryFigure(t *testing.T) {
	minute := entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: summaryDay.Add(5 * time.Hour),
		Open: summaryFigure("1"), High: summaryFigure("2"), Low: summaryFigure("3"), Close: summaryFigure("4"),
		Volume: summaryFigure("5"), QuoteVolume: reportedSummaryFigure("6"),
		TakerBuyBaseVolume: reportedSummaryFigure("7"), TakerBuyQuoteVolume: reportedSummaryFigure("8"),
	}

	summaryDayEntity := domains.NewKCandleSummaryDayDomain(
		"BTCUSDT", summaryDay, 0, []entities.KCandle{minute}).ToEntity()

	require.Len(t, summaryDayEntity.Summaries, 2)
	for _, summary := range summaryDayEntity.Summaries {
		roundTripped := summary.ToKCandle()
		roundTripped.OpenTime = minute.OpenTime
		assert.Equal(t, minute.ToDto(), roundTripped.ToDto(), summary.Interval)
	}
}
