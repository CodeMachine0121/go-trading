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
)

func TestContractAutoOrderRepositoryKeepsTheEntryOrdersProgress(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	queued := testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)
	fillDeadline := autoOrderQueuedAt.Add(15 * time.Minute)
	queued.ReferencePrice = decimal.NewNullDecimal(decimal.NewFromInt(84154))
	queued.EntrySlippageTolerancePercentage = decimal.RequireFromString("0.1")
	queued.EntryFillDeadline = &fillDeadline
	require.NoError(t, testBed.repository.Enqueue(t.Context(), queued))
	order := testBed.storedOrders(t)[0]
	taken, _ := testBed.repository.Claim(t.Context(), order.ID, "replica-a", autoOrderQueuedAt, autoOrderQueuedAt.Add(time.Minute))
	require.True(t, taken)

	placedAt := autoOrderQueuedAt.Add(10 * time.Second)
	order.Status = string(vo.ContractAutoOrderExecuting)
	order.ClaimedBy = "replica-a"
	order.EntryPlacedAt = &placedAt
	order.EntryLimitPrice = decimal.NewNullDecimal(decimal.RequireFromString("84238.1"))
	order.EntryExchangeOrderID = "283194212"
	order.EntryExecutedQuantity = decimal.NewNullDecimal(decimal.RequireFromString("0.001"))
	order.EntryStatus = string(vo.ContractEntryWorking)
	written, saveError := testBed.repository.SaveProgress(t.Context(), order, "replica-a")
	require.NoError(t, saveError)
	require.True(t, written)

	stored := testBed.storedOrders(t)[0]
	assertStoredFigure(t, "84154", stored.ReferencePrice)
	assert.True(t, stored.EntrySlippageTolerancePercentage.Equal(decimal.RequireFromString("0.1")))
	require.NotNil(t, stored.EntryFillDeadline)
	assert.True(t, stored.EntryFillDeadline.Equal(fillDeadline))
	require.NotNil(t, stored.EntryPlacedAt)
	assert.True(t, stored.EntryPlacedAt.Equal(placedAt))
	assertStoredFigure(t, "84238.1", stored.EntryLimitPrice)
	assert.Equal(t, "283194212", stored.EntryExchangeOrderID)
	assertStoredFigure(t, "0.001", stored.EntryExecutedQuantity)
	assert.Equal(t, string(vo.ContractEntryWorking), stored.EntryStatus)
}

func TestContractAutoOrderRepositoryTellsWhetherANewerOrderWaits(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	otherBot, saveError := persistence.NewStrategyBotRepository(testBed.database).Save(t.Context(), aBotRow("另一台"))
	require.NoError(t, saveError)
	settledLater := testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(2*time.Minute), 53)
	settledLater.Status = string(vo.ContractAutoOrderSettled)
	for _, order := range []entities.ContractAutoOrder{
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52),
		settledLater,
		testBed.anOrderOf(otherBot.ID, autoOrderQueuedAt.Add(time.Minute), 7),
	} {
		require.NoError(t, testBed.repository.Enqueue(t.Context(), order))
	}
	head := testBed.storedOrders(t)[0]

	hasLater, findError := testBed.repository.ExistsUnsettledAfter(t.Context(), testBed.botID, "BTCUSDT", head.ID)
	require.NoError(t, findError)
	assert.False(t, hasLater, "a settled order and another bot's order are not waiting behind this one")

	require.NoError(t, testBed.repository.Enqueue(t.Context(),
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(15*time.Minute), 54)))

	hasLater, findError = testBed.repository.ExistsUnsettledAfter(t.Context(), testBed.botID, "BTCUSDT", head.ID)
	require.NoError(t, findError)
	assert.True(t, hasLater)
}

func TestContractAutoOrderRepositoryTellsWhetherAnEntryOrderStillRests(t *testing.T) {
	placedAt := autoOrderQueuedAt.Add(10 * time.Second)
	testCases := []struct {
		name     string
		shape    func(order *entities.ContractAutoOrder)
		expected bool
	}{
		{name: "進場單掛著", shape: func(order *entities.ContractAutoOrder) { order.EntryPlacedAt = &placedAt }, expected: true},
		{name: "進場單還沒送", shape: func(*entities.ContractAutoOrder) {}, expected: false},
		{name: "進場單已收尾", shape: func(order *entities.ContractAutoOrder) {
			order.EntryPlacedAt = &placedAt
			order.OpenDone = true
		}, expected: false},
		{name: "委託已結束", shape: func(order *entities.ContractAutoOrder) {
			order.EntryPlacedAt = &placedAt
			order.Status = string(vo.ContractAutoOrderSettled)
		}, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testBed := newContractAutoOrderTestBed(t)
			order := testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)
			testCase.shape(&order)
			require.NoError(t, testBed.repository.Enqueue(t.Context(), order))

			outstanding, findError := testBed.repository.ExistsOutstandingEntry(t.Context(), testBed.botID)

			require.NoError(t, findError)
			assert.Equal(t, testCase.expected, outstanding)
		})
	}
}

func TestContractAutoOrderRepositoryReportsEntryQuestionsItCannotAsk(t *testing.T) {
	repository := persistence.NewContractAutoOrderRepository(closedDatabase(t))

	_, laterError := repository.ExistsUnsettledAfter(t.Context(), 1, "BTCUSDT", 1)
	_, entryError := repository.ExistsOutstandingEntry(t.Context(), 1)
	_, unsettledError := repository.ExistsUnsettled(t.Context(), 1, "BTCUSDT")

	require.Error(t, laterError)
	require.Error(t, entryError)
	require.Error(t, unsettledError)
}

func TestStrategyBotRepositoryKeepsTheEntrySlippageTolerance(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	withoutOne, saveError := repository.Save(t.Context(), aBotRow("沒設的"))
	require.NoError(t, saveError)
	assert.True(t, withoutOne.AutoOrderEntrySlippageTolerancePercentage.Equal(decimal.RequireFromString("0.1")),
		"a row written without the column reads as the default, as existing rows do: %s",
		withoutOne.AutoOrderEntrySlippageTolerancePercentage)

	zeroRow := aBotRow("零容忍")
	zeroRow.AutoOrderEntrySlippageTolerancePercentage = decimal.RequireFromString("0")
	zero, saveError := repository.Save(t.Context(), zeroRow)
	require.NoError(t, saveError)
	assert.True(t, zero.AutoOrderEntrySlippageTolerancePercentage.IsZero(), "%s", zero.AutoOrderEntrySlippageTolerancePercentage)

	zero.AutoOrderEntrySlippageTolerancePercentage = decimal.RequireFromString("0.3")
	rewritten, rewriteError := repository.Save(t.Context(), zero)
	require.NoError(t, rewriteError)
	assert.True(t, rewritten.AutoOrderEntrySlippageTolerancePercentage.Equal(decimal.RequireFromString("0.3")))
}

func assertStoredFigure(t *testing.T, expected string, figure decimal.NullDecimal) {
	t.Helper()
	require.True(t, figure.Valid, "expected %s, got nothing", expected)
	assert.True(t, figure.Decimal.Equal(decimal.RequireFromString(expected)), "expected %s, got %s", expected, figure.Decimal)
}
