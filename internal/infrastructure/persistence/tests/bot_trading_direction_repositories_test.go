package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// asTheShapeBeforeBotsChoseTheirDirection gives trading strategies back the mode column they held, holding each mode given.
func asTheShapeBeforeBotsChoseTheirDirection(t *testing.T, database *gorm.DB, modeByTradingStrategy map[uint]string) {
	// Raw SQL restores the old column because the entity no longer names it.
	require.NoError(t, database.Exec(
		`ALTER TABLE "TradingStrategies" ADD COLUMN IF NOT EXISTS "contract_trading_mode" varchar(32) NOT NULL DEFAULT ''`).Error)
	for tradingStrategyID, mode := range modeByTradingStrategy {
		require.NoError(t, database.Table("TradingStrategies").
			Where(clause.Eq{Column: "id", Value: tradingStrategyID}).
			UpdateColumn("contract_trading_mode", mode).Error)
	}
}

// aBotFollowing stores a stopped bot of this kind following that trading strategy, holding no direction yet; written directly, as the rows were before the upgrade, so nothing reads the old shape first.
func aBotFollowing(t *testing.T, database *gorm.DB, name string, marketDataKind vo.MarketDataKindVo, tradingStrategyID uint) uint {
	bot := aBotRow(name)
	bot.MarketDataKind = string(marketDataKind)
	bot.TradingStrategyID = tradingStrategyID
	bot.WatchedSymbols = nil
	require.NoError(t, database.Omit(clause.Associations).Create(&bot).Error)

	return bot.ID
}

func TestSchemaMigratorMovesEachTradingStrategysModeOntoTheBotsFollowingIt(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	for _, tradingStrategy := range []entities.TradingStrategy{
		{ID: 21, OwnerID: botRowOwnerID, Name: "只做多", MarketDataKind: "contractKCandle"},
		{ID: 22, OwnerID: botRowOwnerID, Name: "只做空", MarketDataKind: "contractKCandle"},
		{ID: 23, OwnerID: botRowOwnerID, Name: "沒填", MarketDataKind: "contractKCandle"},
	} {
		require.NoError(t, database.Create(&tradingStrategy).Error)
	}
	asTheShapeBeforeBotsChoseTheirDirection(t, database, map[uint]string{21: "longOnly", 22: "shortOnly", 23: ""})
	longBotID := aBotFollowing(t, database, "多", vo.MarketDataKindContractKCandle, 21)
	shortBotID := aBotFollowing(t, database, "空", vo.MarketDataKindContractKCandle, 22)
	blankBotID := aBotFollowing(t, database, "沒填", vo.MarketDataKindContractKCandle, 23)
	spotBotID := aBotFollowing(t, database, "現貨", vo.MarketDataKindKCandle, botRowTradingStrategyID)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	repository := persistence.NewStrategyBotRepository(database)
	testCases := []struct {
		name              string
		botID             uint
		expectedMode      string
		expectedDirection []string
	}{
		{name: "long only becomes facing long", botID: longBotID, expectedMode: "longOnly", expectedDirection: []string{"long"}},
		{name: "short only becomes facing short", botID: shortBotID, expectedMode: "shortOnly", expectedDirection: []string{"short"}},
		{name: "a blank mode becomes facing both", botID: blankBotID, expectedMode: "longShort", expectedDirection: []string{"long", "short"}},
		{name: "a spot bot faces no direction", botID: spotBotID, expectedMode: "", expectedDirection: nil},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bot, findError := repository.FindOne(t.Context(), testCase.botID)
			require.NoError(t, findError)

			assert.Equal(t, testCase.expectedMode, bot.ContractTradingMode)
			assert.Equal(t, testCase.expectedDirection, bot.ToDto().TradingDirections)
			assert.Equal(t, true, bot.ToDto().Notifications.Buy)
			assert.Equal(t, true, bot.ToDto().Notifications.Sell)
			assert.Equal(t, false, bot.ToDto().Notifications.Hold)
		})
	}
	// Kept for this release, so an instance of the previous one still running during a rolling deploy reads the right mode.
	assert.True(t, database.Migrator().HasColumn(&entities.TradingStrategy{}, "contract_trading_mode"))
}

func TestSchemaMigratorLeavesADirectionABotAlreadyHolds(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	require.NoError(t, database.Create(&entities.TradingStrategy{
		ID: 21, OwnerID: botRowOwnerID, Name: "只做多", MarketDataKind: "contractKCandle",
	}).Error)
	asTheShapeBeforeBotsChoseTheirDirection(t, database, map[uint]string{21: "longOnly"})
	botID := aBotFollowing(t, database, "空", vo.MarketDataKindContractKCandle, 21)
	require.NoError(t, database.Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: botID}).UpdateColumn("contract_trading_mode", "shortOnly").Error)

	// Twice, because every replica's start runs it again.
	for range 2 {
		_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
		require.NoError(t, migrateError)
	}

	bot, findError := persistence.NewStrategyBotRepository(database).FindOne(t.Context(), botID)
	require.NoError(t, findError)
	assert.Equal(t, "shortOnly", bot.ContractTradingMode)
}

func TestStrategyBotRepositoryStoresTheDirectionAndNotificationsItIsGiven(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	bot := aBotRow("合約")
	bot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	bot.ContractTradingMode = string(vo.ContractTradingModeShortOnly)
	bot.MutesBuyNotification = true
	bot.SendsHoldNotification = true

	saved, saveError := repository.Save(t.Context(), bot)
	require.NoError(t, saveError)
	created, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, []string{"short"}, created.ToDto().TradingDirections)
	assert.Equal(t, false, created.ToDto().Notifications.Buy, "a false switch is stored, not lost to a column default")
	assert.Equal(t, true, created.ToDto().Notifications.Hold)

	rewrite := created
	rewrite.ContractTradingMode = string(vo.ContractTradingModeLongShort)
	rewrite.MutesBuyNotification = false
	rewrite.MutesSellNotification = true
	rewrite.SendsHoldNotification = false
	_, rewriteError := repository.Save(t.Context(), rewrite)
	require.NoError(t, rewriteError)
	rewritten, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, []string{"long", "short"}, rewritten.ToDto().TradingDirections)
	assert.Equal(t, true, rewritten.ToDto().Notifications.Buy)
	assert.Equal(t, false, rewritten.ToDto().Notifications.Sell)
	assert.Equal(t, false, rewritten.ToDto().Notifications.Hold)
}

func TestStrategyBotRepositoryKeepsEachWatchedSymbolsLastOpinion(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	saved, saveError := repository.Save(t.Context(), aBotRow("記意見"))
	require.NoError(t, saveError)
	stored, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)

	stored.WatchedSymbols[0].LastOpinion = string(vo.SignalHold)
	require.NoError(t, repository.UpdateRunState(t.Context(), stored))

	updated, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "hold", updated.WatchedSymbols[0].LastOpinion)
}

func TestSchemaMigratorReadsAnUnreadableModeAsFacingBoth(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	require.NoError(t, database.Create(&entities.TradingStrategy{
		ID: 21, OwnerID: botRowOwnerID, Name: "壞掉的", MarketDataKind: "contractKCandle",
	}).Error)
	asTheShapeBeforeBotsChoseTheirDirection(t, database, map[uint]string{21: "sideways"})
	botID := aBotFollowing(t, database, "壞", vo.MarketDataKindContractKCandle, 21)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	bot, findError := persistence.NewStrategyBotRepository(database).FindOne(t.Context(), botID)
	require.NoError(t, findError)
	assert.Equal(t, "longShort", bot.ContractTradingMode)
}

// A return-to-hold message given up names its own round, which never carries the signal, so the signal survives.
func TestStrategyBotRepositoryKeepsTheSignalWhenAHoldMessageIsGivenUp(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	saved, saveError := repository.Save(t.Context(), aBotRow("持有"))
	require.NoError(t, saveError)
	stored, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	buyRound := time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)
	holdRound := buyRound.Add(15 * time.Minute)
	stored.WatchedSymbols[0].LastSentSignal = string(vo.SignalBuy)
	stored.WatchedSymbols[0].LastSentRoundDueAt = &buyRound
	stored.WatchedSymbols[0].LastOpinion = string(vo.SignalHold)
	require.NoError(t, repository.UpdateRunState(t.Context(), stored))

	require.NoError(t, repository.ForgetSentSignal(t.Context(), saved.ID, "BTCUSDT", holdRound))

	kept, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "buy", kept.WatchedSymbols[0].LastSentSignal)
}
