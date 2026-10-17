package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	firstOfMarch  = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	secondOfMarch = time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
)

func spotMinuteAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: openTime,
		Open: decimal.RequireFromString("100"), High: decimal.RequireFromString("110"),
		Low: decimal.RequireFromString("90"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString("1"),
	}
}

func spotSummaryAt(interval vo.AggregationIntervalVo, openTime time.Time) entities.KCandleSummary {
	return entities.KCandleSummary{
		Symbol: "BTCUSDT", Interval: string(interval), OpenTime: openTime,
		Open: decimal.RequireFromString("100"), High: decimal.RequireFromString("110"),
		Low: decimal.RequireFromString("90"), Close: decimal.RequireFromString("105"),
		Volume: decimal.RequireFromString("60"),
	}
}

func summarizedSpotDay(day time.Time, revision int64) entities.KCandleSummaryDay {
	return entities.KCandleSummaryDay{
		Symbol: "BTCUSDT", Day: day, Revision: revision,
		Summaries: []entities.KCandleSummary{
			spotSummaryAt(vo.AggregationIntervalOneHour, day.Add(14*time.Hour)),
			spotSummaryAt(vo.AggregationIntervalOneDay, day),
		},
	}
}

// storedSpotDay reads the day back the way a series request would, at the daily interval.
func storedSpotDay(t *testing.T, database *gorm.DB, day time.Time) entities.KCandleSummaryDay {
	summaryDays, findError := persistence.NewKCandleSummaryDayRepository(database).FindDaysInRange(
		t.Context(), "BTCUSDT", vo.AggregationIntervalOneDay, day, day.Add(24*time.Hour))
	require.NoError(t, findError)
	require.Len(t, summaryDays, 1)

	return summaryDays[0]
}

func TestFreshDaySummarizedOnceIsReadBackWithItsSummaries(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)

	isStored, saveError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, saveError)

	assert.True(t, isStored)
	storedDay := storedSpotDay(t, database, firstOfMarch)
	assert.True(t, storedDay.Summarized)
	require.Len(t, storedDay.Summaries, 1)
	assert.Equal(t, string(vo.AggregationIntervalOneDay), storedDay.Summaries[0].Interval)
	assert.True(t, storedDay.Summaries[0].Volume.Equal(decimal.RequireFromString("60")))
}

func TestHourlyReadReturnsOnlyHoursInsideTheRange(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	_, saveError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, saveError)

	summaryDays, findError := summaryDayRepository.FindDaysInRange(t.Context(), "BTCUSDT",
		vo.AggregationIntervalOneHour, firstOfMarch.Add(15*time.Hour), firstOfMarch.Add(24*time.Hour))
	require.NoError(t, findError)

	require.Len(t, summaryDays, 1)
	assert.Empty(t, summaryDays[0].Summaries, "the 14:00 hour lies before the requested 15:00")
}

func TestTwoMergesOfTheSameFreshDayStoreItOnlyOnce(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	_, firstError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, firstError)

	isStored, secondError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, secondError)

	assert.False(t, isStored)
	assert.Len(t, storedSpotDay(t, database, firstOfMarch).Summaries, 1)
}

func TestMinuteWritesInvalidateTheDayTheyTouch(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, kCandleRepository *persistence.KCandleRepository)
	}{
		{"history sync adds a minute", func(t *testing.T, kCandleRepository *persistence.KCandleRepository) {
			storedCount, saveError := kCandleRepository.SaveAllIfAbsent(t.Context(),
				[]entities.KCandle{spotMinuteAt(firstOfMarch.Add(14*time.Hour+5*time.Minute), "101")})
			require.NoError(t, saveError)
			require.Equal(t, 1, storedCount)
		}},
		{"the routine round rewrites a minute", func(t *testing.T, kCandleRepository *persistence.KCandleRepository) {
			_, saveError := kCandleRepository.Save(t.Context(), spotMinuteAt(firstOfMarch.Add(9*time.Hour+50*time.Minute), "102"))
			require.NoError(t, saveError)
		}},
		{"someone edits a minute", func(t *testing.T, kCandleRepository *persistence.KCandleRepository) {
			_, updateError := kCandleRepository.Update(t.Context(), spotMinuteAt(firstOfMarch.Add(8*time.Hour+30*time.Minute), "103"))
			require.NoError(t, updateError)
		}},
		{"someone deletes a minute", func(t *testing.T, kCandleRepository *persistence.KCandleRepository) {
			require.NoError(t, kCandleRepository.Delete(t.Context(), "BTCUSDT", firstOfMarch.Add(8*time.Hour+30*time.Minute)))
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newTestDatabase(t)
			kCandleRepository := persistence.NewKCandleRepository(database)
			_, seedError := kCandleRepository.SaveAllIfAbsent(t.Context(),
				[]entities.KCandle{spotMinuteAt(firstOfMarch.Add(8*time.Hour+30*time.Minute), "100")})
			require.NoError(t, seedError)
			_, saveError := persistence.NewKCandleSummaryDayRepository(database).SaveSummarized(
				t.Context(), summarizedSpotDay(firstOfMarch, 1))
			require.NoError(t, saveError)
			require.True(t, storedSpotDay(t, database, firstOfMarch).Summarized)

			testCase.write(t, kCandleRepository)

			invalidatedDay := storedSpotDay(t, database, firstOfMarch)
			assert.False(t, invalidatedDay.Summarized)
			assert.Empty(t, invalidatedDay.Summaries)
		})
	}
}

func TestHistorySyncThatStoresNothingNewLeavesTheDaySummarized(t *testing.T) {
	database := newTestDatabase(t)
	kCandleRepository := persistence.NewKCandleRepository(database)
	minute := spotMinuteAt(firstOfMarch.Add(8*time.Hour), "100")
	_, seedError := kCandleRepository.SaveAllIfAbsent(t.Context(), []entities.KCandle{minute})
	require.NoError(t, seedError)
	_, saveError := persistence.NewKCandleSummaryDayRepository(database).SaveSummarized(
		t.Context(), summarizedSpotDay(firstOfMarch, 1))
	require.NoError(t, saveError)

	storedCount, syncError := kCandleRepository.SaveAllIfAbsent(t.Context(), []entities.KCandle{minute})
	require.NoError(t, syncError)

	assert.Equal(t, 0, storedCount)
	assert.True(t, storedSpotDay(t, database, firstOfMarch).Summarized)
}

func TestLastMinuteOfADayInvalidatesOnlyThatDay(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	_, firstSaveError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, firstSaveError)
	_, secondSaveError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(secondOfMarch, 0))
	require.NoError(t, secondSaveError)

	_, writeError := persistence.NewKCandleRepository(database).Save(
		t.Context(), spotMinuteAt(firstOfMarch.Add(23*time.Hour+59*time.Minute), "104"))
	require.NoError(t, writeError)

	assert.False(t, storedSpotDay(t, database, firstOfMarch).Summarized)
	assert.True(t, storedSpotDay(t, database, secondOfMarch).Summarized)
	assert.Len(t, storedSpotDay(t, database, secondOfMarch).Summaries, 1)
}

func TestMergeReadBeforeAMinuteChangedIsNotStored(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	kCandleRepository := persistence.NewKCandleRepository(database)
	_, firstWriteError := kCandleRepository.Save(t.Context(), spotMinuteAt(firstOfMarch.Add(time.Hour), "100"))
	require.NoError(t, firstWriteError)
	revisionTheMergeRead := storedSpotDay(t, database, firstOfMarch).Revision

	_, laterWriteError := kCandleRepository.Save(t.Context(), spotMinuteAt(firstOfMarch.Add(2*time.Hour), "101"))
	require.NoError(t, laterWriteError)
	isStored, saveError := summaryDayRepository.SaveSummarized(
		t.Context(), summarizedSpotDay(firstOfMarch, revisionTheMergeRead))
	require.NoError(t, saveError)

	assert.False(t, isStored)
	assert.False(t, storedSpotDay(t, database, firstOfMarch).Summarized)
	isRestored, retryError := summaryDayRepository.SaveSummarized(
		t.Context(), summarizedSpotDay(firstOfMarch, storedSpotDay(t, database, firstOfMarch).Revision))
	require.NoError(t, retryError)
	assert.True(t, isRestored)
}

func TestContractIndexFilledInLaterInvalidatesTheDay(t *testing.T) {
	database := newTestDatabase(t)
	kCandleContractRepository := persistence.NewKCandleContractRepository(database)
	contractSummaryDayRepository := persistence.NewKCandleContractSummaryDayRepository(database)
	openTime := firstOfMarch.Add(3 * time.Hour)
	_, seedError := kCandleContractRepository.Save(
		t.Context(), candleStoredBeforeTheLaterLines("BTCUSDT", openTime, "100"))
	require.NoError(t, seedError)
	summarizedDay := entities.KCandleContractSummaryDay{Symbol: "BTCUSDT", Day: firstOfMarch, Revision: 1}
	isStored, summarizeError := contractSummaryDayRepository.SaveSummarized(t.Context(), summarizedDay)
	require.NoError(t, summarizeError)
	require.True(t, isStored)

	_, fillError := kCandleContractRepository.SaveAllIfAbsent(
		t.Context(), []entities.KCandleContract{contractCandleAt("BTCUSDT", openTime, "100")})
	require.NoError(t, fillError)

	summaryDays, findError := contractSummaryDayRepository.FindDaysInRange(t.Context(), "BTCUSDT",
		vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
	require.NoError(t, findError)
	require.Len(t, summaryDays, 1)
	assert.False(t, summaryDays[0].Summarized)
}

func TestContractDaySummarizedIsReadBackWithItsSummaries(t *testing.T) {
	database := newTestDatabase(t)
	contractSummaryDayRepository := persistence.NewKCandleContractSummaryDayRepository(database)
	summary := entities.KCandleContractSummary{
		Symbol: "BTCUSDT", Interval: string(vo.AggregationIntervalOneDay), OpenTime: firstOfMarch,
		Close: decimal.RequireFromString("105"), TradeCount: 42,
	}

	isStored, saveError := contractSummaryDayRepository.SaveSummarized(t.Context(), entities.KCandleContractSummaryDay{
		Symbol: "BTCUSDT", Day: firstOfMarch, Summaries: []entities.KCandleContractSummary{summary},
	})
	require.NoError(t, saveError)

	assert.True(t, isStored)
	summaryDays, findError := contractSummaryDayRepository.FindDaysInRange(t.Context(), "BTCUSDT",
		vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
	require.NoError(t, findError)
	require.Len(t, summaryDays, 1)
	require.Len(t, summaryDays[0].Summaries, 1)
	assert.Equal(t, int64(42), summaryDays[0].Summaries[0].TradeCount)
	assert.False(t, summaryDays[0].Summaries[0].IndexOpen.Valid)
}

func TestInvalidatedDaySummarizedAgainCarriesTheNewFigures(t *testing.T) {
	database := newTestDatabase(t)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	_, firstSaveError := summaryDayRepository.SaveSummarized(t.Context(), summarizedSpotDay(firstOfMarch, 0))
	require.NoError(t, firstSaveError)
	_, writeError := persistence.NewKCandleRepository(database).Save(
		t.Context(), spotMinuteAt(firstOfMarch.Add(14*time.Hour+5*time.Minute), "101"))
	require.NoError(t, writeError)
	resummarizedDay := summarizedSpotDay(firstOfMarch, storedSpotDay(t, database, firstOfMarch).Revision)
	resummarizedDay.Summaries[1].Volume = decimal.RequireFromString("61")

	isStored, saveError := summaryDayRepository.SaveSummarized(t.Context(), resummarizedDay)
	require.NoError(t, saveError)

	assert.True(t, isStored)
	require.Len(t, storedSpotDay(t, database, firstOfMarch).Summaries, 1)
	assert.True(t, storedSpotDay(t, database, firstOfMarch).Summaries[0].Volume.Equal(decimal.RequireFromString("61")))
}
