package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statisticOf(symbol string, statisticTime time.Time, openInterest string) entities.ContractPositionStatistic {
	return entities.ContractPositionStatistic{
		Symbol:                          symbol,
		StatisticTime:                   statisticTime,
		OpenInterest:                    decimal.RequireFromString(openInterest),
		OpenInterestValue:               decimal.RequireFromString("9262820059.48"),
		AccountLongShare:                decimal.RequireFromString("0.47"),
		AccountShortShare:               decimal.RequireFromString("0.53"),
		AccountLongShortRatio:           decimal.RequireFromString("0.89"),
		TopTraderPositionLongShare:      decimal.RequireFromString("0.6688"),
		TopTraderPositionShortShare:     decimal.RequireFromString("0.3312"),
		TopTraderPositionLongShortRatio: decimal.RequireFromString("2.0189"),
	}
}

func TestContractPositionStatisticRepositoryKeepsTheFirstAnswerForOneMoment(t *testing.T) {
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)
	firstCount, firstError := statisticRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractPositionStatistic{statisticOf("BTCUSDT", at(9, 5), "100")})
	require.NoError(t, firstError)

	secondCount, secondError := statisticRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractPositionStatistic{
			statisticOf("BTCUSDT", at(9, 5), "999"),
			statisticOf("BTCUSDT", at(9, 10), "101"),
		})

	require.NoError(t, secondError)
	assert.Equal(t, 1, firstCount)
	assert.Equal(t, 1, secondCount)
	latest, hasLatest, findError := statisticRepository.FindLatest(t.Context(), "BTCUSDT")
	require.NoError(t, findError)
	require.True(t, hasLatest)
	assert.Equal(t, at(9, 10), latest.StatisticTime.UTC())
	query, _ := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: at(9, 5), EndTime: at(9, 5)})
	held, rangeError := statisticRepository.FindInRange(t.Context(), query, 10)
	require.NoError(t, rangeError)
	require.Len(t, held, 1)
	assert.True(t, decimal.RequireFromString("100").Equal(held[0].OpenInterest))
	assert.True(t, decimal.RequireFromString("2.0189").Equal(held[0].TopTraderPositionLongShortRatio))
}

func TestContractPositionStatisticRepositoryReadsARangeEarliestFirstBothEndsIncluded(t *testing.T) {
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)
	_, saveError := statisticRepository.SaveAllIfAbsent(t.Context(), []entities.ContractPositionStatistic{
		statisticOf("BTCUSDT", at(9, 10), "2"),
		statisticOf("BTCUSDT", at(9, 0), "0"),
		statisticOf("BTCUSDT", at(9, 5), "1"),
		statisticOf("ETHUSDT", at(9, 5), "7"),
		statisticOf("BTCUSDT", at(9, 15), "3"),
	})
	require.NoError(t, saveError)
	query, _ := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 10)})

	held, findError := statisticRepository.FindInRange(t.Context(), query, 10)
	limited, limitedError := statisticRepository.FindInRange(t.Context(), query, 2)

	require.NoError(t, findError)
	require.Len(t, held, 3)
	assert.Equal(t, at(9, 0), held[0].StatisticTime.UTC())
	assert.Equal(t, at(9, 5), held[1].StatisticTime.UTC())
	assert.Equal(t, at(9, 10), held[2].StatisticTime.UTC())
	require.NoError(t, limitedError)
	assert.Len(t, limited, 2)
}

func TestContractPositionStatisticRepositoryHasNoLatestForAContractNeverRecorded(t *testing.T) {
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)

	_, hasLatest, findError := statisticRepository.FindLatest(t.Context(), "BTCUSDT")
	storedCount, saveError := statisticRepository.SaveAllIfAbsent(t.Context(), nil)

	require.NoError(t, findError)
	assert.False(t, hasLatest)
	require.NoError(t, saveError)
	assert.Equal(t, 0, storedCount)
}

func TestContractPositionStatisticRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())
	query, _ := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: at(0, 0), EndTime: at(1, 0)})

	_, saveError := statisticRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractPositionStatistic{statisticOf("BTCUSDT", at(9, 5), "1")})
	_, _, latestError := statisticRepository.FindLatest(t.Context(), "BTCUSDT")
	_, rangeError := statisticRepository.FindInRange(t.Context(), query, 10)

	assert.Error(t, saveError)
	assert.Error(t, latestError)
	assert.Error(t, rangeError)
}
