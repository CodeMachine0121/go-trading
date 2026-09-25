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

func TestContractPositionStatisticRepositoryStoresAThirtyDayCatchUpInOneCall(t *testing.T) {
	// Thirty days is 8640 statistics of ten figures each — more values than one
	// statement can carry.
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)
	thirtyDays := make([]entities.ContractPositionStatistic, 0, 8640)
	firstMoment := at(0, 0).Add(-30 * 24 * time.Hour)
	for index := range 8640 {
		thirtyDays = append(thirtyDays, statisticOf("BTCUSDT", firstMoment.Add(time.Duration(index)*5*time.Minute), "1"))
	}

	storedCount, saveError := statisticRepository.SaveAllIfAbsent(t.Context(), thirtyDays)
	storedAgain, againError := statisticRepository.SaveAllIfAbsent(t.Context(), thirtyDays)

	require.NoError(t, saveError)
	assert.Equal(t, 8640, storedCount)
	require.NoError(t, againError)
	assert.Equal(t, 0, storedAgain)
}

func TestContractPositionStatisticRepositoryCountsOneContractsStatisticsBothEndsIncluded(t *testing.T) {
	database := newTestDatabase(t)
	statisticRepository := persistence.NewContractPositionStatisticRepository(database)
	_, saveError := statisticRepository.SaveAllIfAbsent(t.Context(), []entities.ContractPositionStatistic{
		statisticOf("BTCUSDT", at(9, 0), "100"),
		statisticOf("BTCUSDT", at(9, 5), "101"),
		statisticOf("BTCUSDT", at(9, 10), "102"),
		statisticOf("BTCUSDT", at(9, 15), "103"),
		statisticOf("ETHUSDT", at(9, 5), "200"),
	})
	require.NoError(t, saveError)

	testCases := []struct {
		name          string
		symbol        string
		startTime     time.Time
		endTime       time.Time
		expectedCount int
	}{
		{name: "兩端都算", symbol: "BTCUSDT", startTime: at(9, 5), endTime: at(9, 10), expectedCount: 2},
		{name: "只算這個標的", symbol: "ETHUSDT", startTime: at(9, 0), endTime: at(9, 15), expectedCount: 1},
		{name: "區間外沒有", symbol: "BTCUSDT", startTime: at(10, 0), endTime: at(11, 0), expectedCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			heldCount, countError := statisticRepository.CountInRange(
				t.Context(), testCase.symbol, testCase.startTime, testCase.endTime)

			require.NoError(t, countError)
			assert.Equal(t, testCase.expectedCount, heldCount)
		})
	}
}
