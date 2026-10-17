package persistence_test

import (
	"context"
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

func TestSummaryAndMinuteWritesReportAStorageThatCannotBeReached(t *testing.T) {
	database := newTestDatabase(t)
	kCandleRepository := persistence.NewKCandleRepository(database)
	kCandleContractRepository := persistence.NewKCandleContractRepository(database)
	summaryDayRepository := persistence.NewKCandleSummaryDayRepository(database)
	contractSummaryDayRepository := persistence.NewKCandleContractSummaryDayRepository(database)
	unreachable, cancel := context.WithCancel(t.Context())
	cancel()
	minute := spotMinuteAt(firstOfMarch, "100")
	contractMinute := contractCandleAt("BTCUSDT", firstOfMarch, "100")

	operations := map[string]func() error{
		"save a minute": func() error { _, writeError := kCandleRepository.Save(unreachable, minute); return writeError },
		"save minutes if absent": func() error {
			_, writeError := kCandleRepository.SaveAllIfAbsent(unreachable, []entities.KCandle{minute})
			return writeError
		},
		"update a minute": func() error { _, writeError := kCandleRepository.Update(unreachable, minute); return writeError },
		"delete a minute": func() error { return kCandleRepository.Delete(unreachable, "BTCUSDT", firstOfMarch) },
		"save a contract minute": func() error {
			_, writeError := kCandleContractRepository.Save(unreachable, contractMinute)
			return writeError
		},
		"save contract minutes if absent": func() error {
			_, writeError := kCandleContractRepository.SaveAllIfAbsent(unreachable, []entities.KCandleContract{contractMinute})
			return writeError
		},
		"update a contract minute": func() error {
			_, writeError := kCandleContractRepository.Update(unreachable, contractMinute)
			return writeError
		},
		"delete a contract minute": func() error {
			return kCandleContractRepository.Delete(unreachable, "BTCUSDT", firstOfMarch)
		},
		"find summary days": func() error {
			_, readError := summaryDayRepository.FindDaysInRange(unreachable, "BTCUSDT",
				vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
			return readError
		},
		"summarize a day": func() error {
			_, writeError := summaryDayRepository.SaveSummarized(unreachable, summarizedSpotDay(firstOfMarch, 0))
			return writeError
		},
		"find contract summary days": func() error {
			_, readError := contractSummaryDayRepository.FindDaysInRange(unreachable, "BTCUSDT",
				vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
			return readError
		},
		"summarize a contract day": func() error {
			_, writeError := contractSummaryDayRepository.SaveSummarized(unreachable,
				entities.KCandleContractSummaryDay{Symbol: "BTCUSDT", Day: firstOfMarch})
			return writeError
		},
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, operation())
		})
	}
}

func TestDaySummarizedWithTwoCandlesForOneHourIsNotStored(t *testing.T) {
	database := newTestDatabase(t)
	duplicatedHour := summarizedSpotDay(firstOfMarch, 0)
	duplicatedHour.Summaries = append(duplicatedHour.Summaries, duplicatedHour.Summaries[0])
	contractDuplicatedHour := entities.KCandleContractSummaryDay{Symbol: "BTCUSDT", Day: firstOfMarch,
		Summaries: []entities.KCandleContractSummary{
			{Symbol: "BTCUSDT", Interval: "1h", OpenTime: firstOfMarch},
			{Symbol: "BTCUSDT", Interval: "1h", OpenTime: firstOfMarch},
		}}

	_, spotError := persistence.NewKCandleSummaryDayRepository(database).SaveSummarized(t.Context(), duplicatedHour)
	_, contractError := persistence.NewKCandleContractSummaryDayRepository(database).
		SaveSummarized(t.Context(), contractDuplicatedHour)

	assert.Error(t, spotError)
	assert.Error(t, contractError)
	summaryDays, findError := persistence.NewKCandleSummaryDayRepository(database).FindDaysInRange(
		t.Context(), "BTCUSDT", vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
	require.NoError(t, findError)
	assert.Empty(t, summaryDays, "the claim rolls back with the failed summaries")
}

func TestMinuteWritesTheStorageRefusesLeaveTheDaySummarized(t *testing.T) {
	database := newTestDatabase(t)
	kCandleRepository := persistence.NewKCandleRepository(database)
	kCandleContractRepository := persistence.NewKCandleContractRepository(database)
	_, seedError := kCandleRepository.Save(t.Context(), spotMinuteAt(firstOfMarch, "100"))
	require.NoError(t, seedError)
	_, contractSeedError := kCandleContractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", firstOfMarch, "100"))
	require.NoError(t, contractSeedError)
	_, summarizeError := persistence.NewKCandleSummaryDayRepository(database).SaveSummarized(
		t.Context(), summarizedSpotDay(firstOfMarch, storedSpotDay(t, database, firstOfMarch).Revision))
	require.NoError(t, summarizeError)
	// Twenty-one integer digits overflow numeric(38,18), so the storage refuses the minute itself.
	overflowing := decimal.RequireFromString("123456789012345678901")
	spotMinute := spotMinuteAt(firstOfMarch, "100")
	spotMinute.Close = overflowing
	contractMinute := contractCandleAt("BTCUSDT", firstOfMarch, "100")
	contractMinute.Close = overflowing

	refusedWrites := map[string]func() error{
		"save a minute": func() error { _, writeError := kCandleRepository.Save(t.Context(), spotMinute); return writeError },
		"save new minutes": func() error {
			_, writeError := kCandleRepository.SaveAllIfAbsent(t.Context(),
				[]entities.KCandle{spotMinuteAt(firstOfMarch.Add(time.Hour), "123456789012345678901")})
			return writeError
		},
		"update a minute": func() error { _, writeError := kCandleRepository.Update(t.Context(), spotMinute); return writeError },
		"save a contract minute": func() error {
			_, writeError := kCandleContractRepository.Save(t.Context(), contractMinute)
			return writeError
		},
		"save new contract minutes": func() error {
			overflowingNew := contractCandleAt("BTCUSDT", firstOfMarch.Add(time.Hour), "100")
			overflowingNew.Close = overflowing
			_, writeError := kCandleContractRepository.SaveAllIfAbsent(t.Context(), []entities.KCandleContract{overflowingNew})
			return writeError
		},
		"update a contract minute": func() error {
			_, writeError := kCandleContractRepository.Update(t.Context(), contractMinute)
			return writeError
		},
	}

	for name, refusedWrite := range refusedWrites {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, refusedWrite())
			assert.True(t, storedSpotDay(t, database, firstOfMarch).Summarized)
		})
	}
}

func TestContractMergeReadBeforeAMinuteChangedIsNotStored(t *testing.T) {
	database := newTestDatabase(t)
	kCandleContractRepository := persistence.NewKCandleContractRepository(database)
	contractSummaryDayRepository := persistence.NewKCandleContractSummaryDayRepository(database)
	_, firstWriteError := kCandleContractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", firstOfMarch, "100"))
	require.NoError(t, firstWriteError)
	_, laterWriteError := kCandleContractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", firstOfMarch.Add(time.Hour), "101"))
	require.NoError(t, laterWriteError)

	isStored, saveError := contractSummaryDayRepository.SaveSummarized(t.Context(),
		entities.KCandleContractSummaryDay{Symbol: "BTCUSDT", Day: firstOfMarch, Revision: 1})
	require.NoError(t, saveError)

	assert.False(t, isStored, "two writes moved the revision past the one the merge read")
	summaryDays, findError := contractSummaryDayRepository.FindDaysInRange(t.Context(), "BTCUSDT",
		vo.AggregationIntervalOneDay, firstOfMarch, secondOfMarch)
	require.NoError(t, findError)
	require.Len(t, summaryDays, 1)
	assert.False(t, summaryDays[0].Summarized)
	assert.Equal(t, int64(2), summaryDays[0].Revision)
}
