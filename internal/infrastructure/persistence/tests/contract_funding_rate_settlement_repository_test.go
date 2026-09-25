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

func settlementOf(symbol string, settlementTime time.Time, rate string) entities.ContractFundingRateSettlement {
	return entities.ContractFundingRateSettlement{
		Symbol:         symbol,
		SettlementTime: settlementTime,
		FundingRate:    decimal.RequireFromString(rate),
		MarkPrice:      storedFigure("87000"),
	}
}

func TestContractFundingRateSettlementRepositoryKeepsTheFirstAnswerForOneSettlement(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	firstCount, firstError := settlementRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractFundingRateSettlement{settlementOf("BTCUSDT", at(8, 0), "0.0001")})
	require.NoError(t, firstError)

	secondCount, secondError := settlementRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractFundingRateSettlement{
			settlementOf("BTCUSDT", at(8, 0), "0.0009"),
			settlementOf("BTCUSDT", at(16, 0), "-0.00003"),
		})

	require.NoError(t, secondError)
	assert.Equal(t, 1, firstCount)
	assert.Equal(t, 1, secondCount)
	latest, hasLatest, findError := settlementRepository.FindLatest(t.Context(), "BTCUSDT")
	require.NoError(t, findError)
	require.True(t, hasLatest)
	assert.Equal(t, at(16, 0), latest.SettlementTime.UTC())
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(8, 0), EndTime: at(8, 0),
	})
	require.NoError(t, queryError)
	held, rangeError := settlementRepository.FindInRange(t.Context(), query, 10)
	require.NoError(t, rangeError)
	require.Len(t, held, 1)
	assert.True(t, decimal.RequireFromString("0.0001").Equal(held[0].FundingRate))
}

func TestContractFundingRateSettlementRepositoryReadsARangeEarliestFirstBothEndsIncluded(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	withoutMarkPrice := settlementOf("BTCUSDT", at(0, 0), "0.0001")
	withoutMarkPrice.MarkPrice = decimal.NullDecimal{}
	_, saveError := settlementRepository.SaveAllIfAbsent(t.Context(), []entities.ContractFundingRateSettlement{
		settlementOf("BTCUSDT", at(16, 0), "0.0003"),
		withoutMarkPrice,
		settlementOf("BTCUSDT", at(8, 0), "0.0002"),
		settlementOf("ETHUSDT", at(8, 0), "0.0005"),
		settlementOf("BTCUSDT", at(23, 0), "0.0004"),
	})
	require.NoError(t, saveError)
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(0, 0), EndTime: at(16, 0),
	})
	require.NoError(t, queryError)

	held, findError := settlementRepository.FindInRange(t.Context(), query, 10)

	require.NoError(t, findError)
	require.Len(t, held, 3)
	assert.Equal(t, at(0, 0), held[0].SettlementTime.UTC())
	assert.False(t, held[0].MarkPrice.Valid)
	assert.Equal(t, at(8, 0), held[1].SettlementTime.UTC())
	assert.Equal(t, at(16, 0), held[2].SettlementTime.UTC())
	limited, limitedError := settlementRepository.FindInRange(t.Context(), query, 2)
	require.NoError(t, limitedError)
	assert.Len(t, limited, 2)
}

func TestContractFundingRateSettlementRepositoryKeepsTheMillisecond(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	settledAt := at(8, 0).Add(time.Millisecond)
	_, saveError := settlementRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractFundingRateSettlement{settlementOf("BTCUSDT", settledAt, "0.0001")})
	require.NoError(t, saveError)

	latest, _, findError := settlementRepository.FindLatest(t.Context(), "BTCUSDT")

	require.NoError(t, findError)
	assert.Equal(t, settledAt, latest.SettlementTime.UTC())
}

func TestContractFundingRateSettlementRepositoryHasNoLatestForAContractNeverSettled(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)

	_, hasLatest, findError := settlementRepository.FindLatest(t.Context(), "BTCUSDT")
	storedCount, saveError := settlementRepository.SaveAllIfAbsent(t.Context(), nil)

	require.NoError(t, findError)
	assert.False(t, hasLatest)
	require.NoError(t, saveError)
	assert.Equal(t, 0, storedCount)
}

func TestContractFundingRateSettlementRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())
	query, _ := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(0, 0), EndTime: at(1, 0),
	})

	_, saveError := settlementRepository.SaveAllIfAbsent(t.Context(),
		[]entities.ContractFundingRateSettlement{settlementOf("BTCUSDT", at(8, 0), "0.0001")})
	_, _, latestError := settlementRepository.FindLatest(t.Context(), "BTCUSDT")
	_, rangeError := settlementRepository.FindInRange(t.Context(), query, 10)

	assert.Error(t, saveError)
	assert.Error(t, latestError)
	assert.Error(t, rangeError)
}

func TestContractFundingRateSettlementRepositoryStoresAHistoryLongerThanOneStatementCarries(t *testing.T) {
	// 17000 four-column settlements exceed one statement's parameter limit.
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	longHistory := make([]entities.ContractFundingRateSettlement, 0, 17000)
	firstSettlement := at(0, 0).Add(-17000 * time.Hour)
	for index := range 17000 {
		longHistory = append(longHistory, settlementOf("BTCUSDT", firstSettlement.Add(time.Duration(index)*time.Hour), "0.0001"))
	}

	storedCount, saveError := settlementRepository.SaveAllIfAbsent(t.Context(), longHistory)

	require.NoError(t, saveError)
	assert.Equal(t, 17000, storedCount)
}

func TestContractFundingRateSettlementRepositoryFindsTheLatestBeforeACutOff(t *testing.T) {
	database := newTestDatabase(t)
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(database)
	_, saveError := settlementRepository.SaveAllIfAbsent(t.Context(), []entities.ContractFundingRateSettlement{
		settlementOf("BTCUSDT", at(0, 0), "0.0001"),
		settlementOf("BTCUSDT", at(8, 0), "0.0002"),
		settlementOf("ETHUSDT", at(7, 0), "0.0009"),
	})
	require.NoError(t, saveError)

	testCases := []struct {
		name         string
		cutoffTime   time.Time
		expectedRate string
		expectedFind bool
	}{
		{name: "the latest one strictly before the cut-off", cutoffTime: at(9, 0), expectedRate: "0.0002", expectedFind: true},
		{name: "one stamped exactly at the cut-off is left out", cutoffTime: at(8, 0), expectedRate: "0.0001", expectedFind: true},
		{name: "none before the cut-off is not a failure", cutoffTime: at(0, 0), expectedFind: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			latest, found, findError := settlementRepository.FindLatestBefore(t.Context(), "BTCUSDT", testCase.cutoffTime)

			require.NoError(t, findError)
			require.Equal(t, testCase.expectedFind, found)
			if testCase.expectedFind {
				assert.True(t, decimal.RequireFromString(testCase.expectedRate).Equal(latest.FundingRate))
				assert.Equal(t, "BTCUSDT", latest.Symbol)
			}
		})
	}
}

func TestContractFundingRateSettlementRepositoryFindLatestBeforeReportsAStorageFailure(t *testing.T) {
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(closedDatabase(t))

	_, _, findError := settlementRepository.FindLatestBefore(t.Context(), "BTCUSDT", at(9, 0))

	assert.Error(t, findError)
}
