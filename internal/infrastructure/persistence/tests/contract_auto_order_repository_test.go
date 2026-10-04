package persistence_test

import (
	"context"
	"sync"
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

var autoOrderQueuedAt = time.Date(2026, 10, 4, 6, 30, 0, 0, time.UTC)

type contractAutoOrderTestBed struct {
	database   *gorm.DB
	repository *persistence.ContractAutoOrderRepository
	botID      uint
}

func newContractAutoOrderTestBed(t *testing.T) contractAutoOrderTestBed {
	t.Helper()

	database := newStrategyBotTestDatabase(t)
	bot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), aBotRow("合約突破"))
	require.NoError(t, saveError)

	return contractAutoOrderTestBed{
		database: database, repository: persistence.NewContractAutoOrderRepository(database), botID: bot.ID,
	}
}

func (testBed contractAutoOrderTestBed) anOrderOf(botID uint, roundDueAt time.Time, runNumber int) entities.ContractAutoOrder {
	return entities.ContractAutoOrder{
		StrategyBotID: botID, RoundDueAt: roundDueAt, RunNumber: runNumber, OwnerUserID: botRowOwnerID,
		Symbol: "BTCUSDT", TargetPosition: "long", OpenQuantity: decimal.NewNullDecimal(decimal.RequireFromString("0.002")),
		Leverage: decimal.NewFromInt(3), Status: string(vo.ContractAutoOrderReady),
		NextAttemptAt: autoOrderQueuedAt, ExpiresAt: autoOrderQueuedAt.Add(2 * time.Minute), CreatedAt: autoOrderQueuedAt,
	}
}

func (testBed contractAutoOrderTestBed) storedOrders(t *testing.T) []entities.ContractAutoOrder {
	t.Helper()

	stored := []entities.ContractAutoOrder{}
	require.NoError(t, testBed.database.WithContext(t.Context()).Order("id ASC").Find(&stored).Error)

	return stored
}

func TestContractAutoOrderRepositoryQueuesOneOrderARound(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)

	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))

	assert.Len(t, testBed.storedOrders(t), 1)
}

func TestContractAutoOrderRepositoryOffersOnlyEachBotsOldestUnsettledOrder(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	otherBot, saveError := persistence.NewStrategyBotRepository(testBed.database).Save(t.Context(), aBotRow("另一台"))
	require.NoError(t, saveError)
	settled := testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(-time.Hour), 50)
	settled.Status = string(vo.ContractAutoOrderSettled)
	for _, order := range []entities.ContractAutoOrder{
		settled,
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 51),
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(time.Minute), 52),
		testBed.anOrderOf(otherBot.ID, autoOrderQueuedAt, 7),
	} {
		require.NoError(t, testBed.repository.Enqueue(t.Context(), order))
	}

	candidates, findError := testBed.repository.FindDispatchCandidates(t.Context(), 10)

	require.NoError(t, findError)
	runNumbers := []int{}
	for _, candidate := range candidates {
		runNumbers = append(runNumbers, candidate.RunNumber)
	}
	assert.ElementsMatch(t, []int{51, 7}, runNumbers)
}

func TestContractAutoOrderRepositoryLetsExactlyOneReplicaTakeAnOrder(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))
	orderID := testBed.storedOrders(t)[0].ID

	takenCount := 0
	takenMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	for replica := range 8 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			taken, claimError := testBed.repository.Claim(context.Background(), orderID,
				string(rune('a'+replica)), autoOrderQueuedAt, autoOrderQueuedAt.Add(2*time.Minute))
			assert.NoError(t, claimError)
			if taken {
				takenMutex.Lock()
				defer takenMutex.Unlock()
				takenCount++
			}
		}()
	}
	waitGroup.Wait()

	assert.Equal(t, 1, takenCount)
}

func TestContractAutoOrderRepositoryHandsAnOrderOnOnceItsClaimRunsOut(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))
	orderID := testBed.storedOrders(t)[0].ID
	claimedUntil := autoOrderQueuedAt.Add(2 * time.Minute)

	taken, _ := testBed.repository.Claim(t.Context(), orderID, "replica-a", autoOrderQueuedAt, claimedUntil)
	require.True(t, taken)
	takenWhileHeld, _ := testBed.repository.Claim(t.Context(), orderID, "replica-b", claimedUntil.Add(-time.Second), claimedUntil)
	takenAfter, _ := testBed.repository.Claim(t.Context(), orderID, "replica-b", claimedUntil, claimedUntil.Add(time.Minute))

	assert.False(t, takenWhileHeld)
	assert.True(t, takenAfter)
}

func TestContractAutoOrderRepositoryWritesProgressOnlyForItsHolder(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))
	order := testBed.storedOrders(t)[0]
	taken, _ := testBed.repository.Claim(t.Context(), order.ID, "replica-a", autoOrderQueuedAt, autoOrderQueuedAt.Add(time.Minute))
	require.True(t, taken)

	settledAt := autoOrderQueuedAt.Add(20 * time.Second)
	order.Status = string(vo.ContractAutoOrderSettled)
	order.OpenDone = true
	order.OpenedQuantity = decimal.NewNullDecimal(decimal.RequireFromString("0.002"))
	order.Outcome = string(vo.ContractAutoOrderFilled)
	order.ClaimedBy = ""
	order.ClaimedUntil = nil
	order.SettledAt = &settledAt

	writtenByStranger, strangerError := testBed.repository.SaveProgress(t.Context(), order, "replica-b")
	require.NoError(t, strangerError)
	assert.False(t, writtenByStranger)
	assert.Empty(t, testBed.storedOrders(t)[0].Outcome)

	written, saveError := testBed.repository.SaveProgress(t.Context(), order, "replica-a")
	require.NoError(t, saveError)
	assert.True(t, written)
	stored := testBed.storedOrders(t)[0]
	assert.Equal(t, string(vo.ContractAutoOrderFilled), stored.Outcome)
	assert.True(t, stored.OpenDone)
	assert.Empty(t, stored.ClaimedBy)
	assert.Nil(t, stored.ClaimedUntil)
}

func TestContractAutoOrderRepositoryFindsTheOrdersOfTheGivenRounds(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	for runNumber := 50; runNumber <= 53; runNumber++ {
		require.NoError(t, testBed.repository.Enqueue(t.Context(),
			testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(time.Duration(runNumber)*time.Minute), runNumber)))
	}

	found, findError := testBed.repository.FindByBotRunNumbers(t.Context(), testBed.botID, []int{51, 53, 99})

	require.NoError(t, findError)
	runNumbers := []int{}
	for _, order := range found {
		runNumbers = append(runNumbers, order.RunNumber)
	}
	assert.ElementsMatch(t, []int{51, 53}, runNumbers)
}

func TestDeletingABotDropsTheOrdersItHasNotCarriedOut(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))

	require.NoError(t, persistence.NewStrategyBotRepository(testBed.database).Delete(t.Context(), testBed.botID))

	assert.Empty(t, testBed.storedOrders(t))
}

func TestStrategyBotRepositoryWritesOnlyTheBotsOwnPosition(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	repository := persistence.NewStrategyBotRepository(testBed.database)

	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), testBed.botID, vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionLong, Quantity: decimal.RequireFromString("0.002"),
		StopLossClientID: "gt-ao-41-sl", TakeProfitClientID: "gt-ao-41-tp",
	}))
	held, _ := repository.FindOne(t.Context(), testBed.botID)
	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), testBed.botID, vo.AutoOrderPositionVo{}))
	flat, _ := repository.FindOne(t.Context(), testBed.botID)

	assert.Equal(t, "long", held.AutoOrderPositionDirection)
	assert.True(t, held.AutoOrderPositionQuantity.Equal(decimal.RequireFromString("0.002")))
	assert.Equal(t, "gt-ao-41-sl", held.AutoOrderStopLossClientID)
	assert.Equal(t, "gt-ao-41-tp", held.AutoOrderTakeProfitClientID)
	assert.Equal(t, "合約突破", held.Name)
	assert.Empty(t, flat.AutoOrderPositionDirection)
	assert.True(t, flat.AutoOrderPositionQuantity.IsZero())
	assert.Empty(t, flat.AutoOrderStopLossClientID)
}

func TestARewriteOfTheBotLeavesItsOwnPositionAlone(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	repository := persistence.NewStrategyBotRepository(testBed.database)
	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), testBed.botID, vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionShort, Quantity: decimal.RequireFromString("0.003"),
	}))
	bot, _ := repository.FindOne(t.Context(), testBed.botID)

	bot.Name = "改名了"
	_, saveError := repository.Save(t.Context(), bot)
	require.NoError(t, saveError)
	require.NoError(t, repository.UpdateRunState(t.Context(), bot))

	rewritten, _ := repository.FindOne(t.Context(), testBed.botID)
	assert.Equal(t, "改名了", rewritten.Name)
	assert.Equal(t, "short", rewritten.AutoOrderPositionDirection)
	assert.True(t, rewritten.AutoOrderPositionQuantity.Equal(decimal.RequireFromString("0.003")))
}

func TestDisablingAutoOrderByOwnerStaysWithinTheKindsGiven(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約", string(vo.MarketDataKindContractKCandle))
	spotBot := aSwitchedOnBot(t, database, botRowOwnerID, "現貨", string(vo.MarketDataKindKCandle))
	strangersBot := aSwitchedOnBot(t, database, otherBotOwnerID, "別人的", string(vo.MarketDataKindContractKCandle))

	require.NoError(t, repository.DisableAutoOrderByOwner(t.Context(), botRowOwnerID,
		[]string{string(vo.MarketDataKindContractKCandle)}))

	assert.False(t, autoOrderOf(t, database, contractBot))
	assert.True(t, autoOrderOf(t, database, spotBot))
	assert.True(t, autoOrderOf(t, database, strangersBot))
}

func TestDisablingAutoOrderByOwnerWithNoKindsSwitchesOffAllOfTheirs(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約", string(vo.MarketDataKindContractKCandle))
	spotBot := aSwitchedOnBot(t, database, botRowOwnerID, "現貨", string(vo.MarketDataKindKCandle))
	strangersBot := aSwitchedOnBot(t, database, otherBotOwnerID, "別人的", string(vo.MarketDataKindKCandle))

	require.NoError(t, repository.DisableAutoOrderByOwner(t.Context(), botRowOwnerID, nil))

	assert.False(t, autoOrderOf(t, database, contractBot))
	assert.False(t, autoOrderOf(t, database, spotBot))
	assert.True(t, autoOrderOf(t, database, strangersBot))
}

func TestContractAutoOrderRepositoryReportsStorageItCannotReach(t *testing.T) {
	repository := persistence.NewContractAutoOrderRepository(closedDatabase(t))
	botRepository := persistence.NewStrategyBotRepository(closedDatabase(t))
	anOrder := entities.ContractAutoOrder{ID: 1, StrategyBotID: 1, RoundDueAt: autoOrderQueuedAt}

	_, candidatesError := repository.FindDispatchCandidates(t.Context(), 10)
	_, claimError := repository.Claim(t.Context(), 1, "replica-a", autoOrderQueuedAt, autoOrderQueuedAt)
	_, saveError := repository.SaveProgress(t.Context(), anOrder, "replica-a")
	_, findError := repository.FindByBotRunNumbers(t.Context(), 1, []int{52})

	require.Error(t, repository.Enqueue(t.Context(), anOrder))
	require.Error(t, candidatesError)
	require.Error(t, claimError)
	require.Error(t, saveError)
	require.Error(t, findError)
	require.Error(t, botRepository.DisableAutoOrderByOwner(t.Context(), 1, nil))
	require.Error(t, botRepository.UpdateAutoOrderPosition(t.Context(), 1, vo.AutoOrderPositionVo{}))
}

func TestContractAutoOrderRepositoryFindsNothingForNoRounds(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 52)))

	found, findError := testBed.repository.FindByBotRunNumbers(t.Context(), testBed.botID, nil)

	require.NoError(t, findError)
	assert.Empty(t, found)
}

func TestContractAutoOrderRepositoryOffersNothingWhenNothingWaits(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)

	candidates, findError := testBed.repository.FindDispatchCandidates(t.Context(), 10)

	require.NoError(t, findError)
	assert.Empty(t, candidates)
}

func TestAFlatPositionIsStoredWithoutADirection(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	repository := persistence.NewStrategyBotRepository(testBed.database)

	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), testBed.botID, vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionFlat, Quantity: decimal.Zero,
	}))

	stored, _ := repository.FindOne(t.Context(), testBed.botID)
	assert.Empty(t, stored.AutoOrderPositionDirection)
}
