package persistence_test

import (
	"os"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aBotSharing saves a bot whose symbols hold the given shares, as symbol and percentage pairs.
func aBotSharing(t *testing.T, database *gorm.DB, name string, symbolsAndPercentages ...string) entities.StrategyBot {
	t.Helper()

	bot := aBotRow(name)
	bot.WatchedSymbols = []entities.StrategyBotWatchedSymbol{}
	for index := 0; index < len(symbolsAndPercentages); index += 2 {
		bot.WatchedSymbols = append(bot.WatchedSymbols, entities.StrategyBotWatchedSymbol{
			Symbol:               symbolsAndPercentages[index],
			AllocationPercentage: decimal.RequireFromString(symbolsAndPercentages[index+1]),
		})
	}

	savedBot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), bot)
	require.NoError(t, saveError)

	return savedBot
}

func sharesHeldBy(t *testing.T, database *gorm.DB, botID uint) map[string]string {
	t.Helper()

	bot, findError := persistence.NewStrategyBotRepository(database).FindOne(t.Context(), botID)
	require.NoError(t, findError)

	shares := map[string]string{}
	for _, watchedSymbol := range bot.WatchedSymbols {
		shares[watchedSymbol.Symbol] = watchedSymbol.AllocationPercentage.String()
	}

	return shares
}

func TestStrategyBotRepositoryRewriteMovesTheSharesButNothingElse(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	savedBot := aBotSharing(t, database, "分配", "BTCUSDT", "60", "ETHUSDT", "40")
	assert.Equal(t, map[string]string{"BTCUSDT": "60", "ETHUSDT": "40"}, sharesHeldBy(t, database, savedBot.ID))
	savedBot.WatchedSymbols[0].LastSentSignal = string(vo.SignalBuy)
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))

	rewritten := aBotRow("分配")
	rewritten.ID = savedBot.ID
	rewritten.WatchedSymbols = []entities.StrategyBotWatchedSymbol{
		{Symbol: "BTCUSDT", AllocationPercentage: decimal.NewFromInt(20)},
		{Symbol: "ETHUSDT", AllocationPercentage: decimal.RequireFromString("33.33")},
	}
	rewrittenBot, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, map[string]string{"BTCUSDT": "20", "ETHUSDT": "33.33"}, sharesHeldBy(t, database, savedBot.ID))
	bitcoin, _ := rewrittenBot.WatchedSymbol("BTCUSDT")
	assert.Equal(t, string(vo.SignalBuy), bitcoin.LastSentSignal)
}

// Before shares existed every symbol used the whole plan; the upgrade splits bots watching several symbols and touches nothing valid.
func TestSchemaMigratorSharesOutTheCapitalOfBotsFromBeforeShares(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	// Shaped through its own connections, since a statement cached before the column comes back would be refused once its result type changes.
	shapingDatabase, openError := persistence.NewDatabase(os.Getenv("TEST_POSTGRES_DSN"))
	require.NoError(t, openError)
	oneSymbol := aBotSharing(t, shapingDatabase, "單一", "BTCUSDT", "100")
	threeSymbols := aBotSharing(t, shapingDatabase, "三個", "BTCUSDT", "100", "ETHUSDT", "100", "SOLUSDT", "100")
	ownersShares := aBotSharing(t, shapingDatabase, "自訂", "BTCUSDT", "70", "ETHUSDT", "30")
	require.NoError(t, shapingDatabase.Migrator().DropColumn(&entities.StrategyBotWatchedSymbol{}, "allocation_percentage"))
	shapingConnections, connectionError := shapingDatabase.DB()
	require.NoError(t, connectionError)
	require.NoError(t, shapingConnections.Close())

	// Twice, because every replica's start runs it again.
	for range 2 {
		_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
		require.NoError(t, migrateError)

		assert.Equal(t, map[string]string{"BTCUSDT": "100"}, sharesHeldBy(t, database, oneSymbol.ID))
		assert.Equal(t, map[string]string{"BTCUSDT": "33.33", "ETHUSDT": "33.33", "SOLUSDT": "33.33"},
			sharesHeldBy(t, database, threeSymbols.ID))
	}

	rewritten := aBotRow("自訂")
	rewritten.ID = ownersShares.ID
	rewritten.WatchedSymbols = []entities.StrategyBotWatchedSymbol{
		{Symbol: "BTCUSDT", AllocationPercentage: decimal.NewFromInt(70)},
		{Symbol: "ETHUSDT", AllocationPercentage: decimal.NewFromInt(30)},
	}
	_, rewriteError := persistence.NewStrategyBotRepository(database).Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	assert.Equal(t, map[string]string{"BTCUSDT": "70", "ETHUSDT": "30"}, sharesHeldBy(t, database, ownersShares.ID))
}
