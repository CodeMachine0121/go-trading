package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const otherBotOwnerID = uint(2)

func aTradingKeyOf(userID uint, tail string, spot bool, contract bool) entities.BinanceTradingKey {
	return entities.BinanceTradingKey{
		UserID: userID, SealedApiKey: "sealed-api-" + tail, SealedSecretKey: "sealed-secret-" + tail,
		ApiKeyTail: tail, SpotTradingEnabled: spot, ContractTradingEnabled: contract,
	}
}

// newTradingKeyTestDatabase adds a second owner, so switching off can be shown to stay within one person's bots.
func newTradingKeyTestDatabase(t *testing.T) *gorm.DB {
	database := newStrategyBotTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: otherBotOwnerID, Email: "someone-else@example.com", PasswordProof: "a-proof",
	}).Error)

	return database
}

// aSwitchedOnBot stores a bot with auto order on directly, since only the switch routes may turn it on.
func aSwitchedOnBot(t *testing.T, database *gorm.DB, ownerID uint, name string, marketDataKind string) uint {
	bot := aBotRow(name)
	bot.OwnerID = ownerID
	bot.MarketDataKind = marketDataKind
	require.NoError(t, database.WithContext(t.Context()).Omit(clause.Associations).Create(&bot).Error)
	require.NoError(t, database.WithContext(t.Context()).Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: bot.ID}).
		UpdateColumn("market_data_kind", marketDataKind).Error)
	require.NoError(t, database.WithContext(t.Context()).Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: bot.ID}).
		UpdateColumn("auto_order_enabled", true).Error)

	return bot.ID
}

func autoOrderOf(t *testing.T, database *gorm.DB, botID uint) bool {
	bot, findError := persistence.NewStrategyBotRepository(database).FindOne(t.Context(), botID)
	require.NoError(t, findError)

	return bot.AutoOrderEnabled
}

func TestBinanceTradingKeyRepositoryReplaceKeepsOneKeyPerPerson(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	repository := persistence.NewBinanceTradingKeyRepository(database)

	firstKey, firstError := repository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, false), nil)
	require.NoError(t, firstError)
	assert.Positive(t, firstKey.ID)
	_, secondError := repository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "c3d4", true, true), nil)
	require.NoError(t, secondError)

	storedKey, findError := repository.FindOneByUser(t.Context(), botRowOwnerID)
	require.NoError(t, findError)
	assert.Equal(t, "c3d4", storedKey.ApiKeyTail)
	assert.Equal(t, "sealed-secret-c3d4", storedKey.SealedSecretKey)
	assert.True(t, storedKey.ContractTradingEnabled)
	assert.False(t, storedKey.UpdatedAt.IsZero())

	keyCount := int64(0)
	require.NoError(t, database.Model(&entities.BinanceTradingKey{}).
		Where(clause.Eq{Column: "user_id", Value: botRowOwnerID}).Count(&keyCount).Error)
	assert.Equal(t, int64(1), keyCount)
}

func TestBinanceTradingKeyRepositoryFindOneByUserSaysNothingIsThere(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	repository := persistence.NewBinanceTradingKeyRepository(database)
	_, replaceError := repository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, false), nil)
	require.NoError(t, replaceError)

	_, findError := repository.FindOneByUser(t.Context(), otherBotOwnerID)

	require.ErrorIs(t, findError, domains.ErrBinanceTradingKeyNotConfigured)
}

func TestBinanceTradingKeyRepositoryReplaceSwitchesOffOnlyTheOwnersUncoveredBots(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約甲", string(vo.MarketDataKindContractKCandle))
	spotBot := aSwitchedOnBot(t, database, botRowOwnerID, "現貨乙", string(vo.MarketDataKindKCandle))
	strangersContractBot := aSwitchedOnBot(t, database, otherBotOwnerID, "別人的合約", string(vo.MarketDataKindContractKCandle))

	_, replaceError := persistence.NewBinanceTradingKeyRepository(database).Replace(t.Context(),
		aTradingKeyOf(botRowOwnerID, "c3d4", true, false),
		domains.NewTradableMarketsDomain(true, false).UncoveredBotMarketDataKinds())

	require.NoError(t, replaceError)
	assert.False(t, autoOrderOf(t, database, contractBot))
	assert.True(t, autoOrderOf(t, database, spotBot))
	assert.True(t, autoOrderOf(t, database, strangersContractBot))
}

func TestBinanceTradingKeyRepositoryReplaceSwitchesOffOldBlankKindSpotBots(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	oldSpotBot := aSwitchedOnBot(t, database, botRowOwnerID, "舊現貨", "")
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約甲", string(vo.MarketDataKindContractKCandle))

	_, replaceError := persistence.NewBinanceTradingKeyRepository(database).Replace(t.Context(),
		aTradingKeyOf(botRowOwnerID, "c3d4", false, true),
		domains.NewTradableMarketsDomain(false, true).UncoveredBotMarketDataKinds())

	require.NoError(t, replaceError)
	assert.False(t, autoOrderOf(t, database, oldSpotBot))
	assert.True(t, autoOrderOf(t, database, contractBot))
}

func TestBinanceTradingKeyRepositoryDeleteByUserSwitchesEveryOneOfTheirBotsOff(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	repository := persistence.NewBinanceTradingKeyRepository(database)
	_, replaceError := repository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, true), nil)
	require.NoError(t, replaceError)
	spotBot := aSwitchedOnBot(t, database, botRowOwnerID, "現貨乙", string(vo.MarketDataKindKCandle))
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約甲", string(vo.MarketDataKindContractKCandle))
	strangersBot := aSwitchedOnBot(t, database, otherBotOwnerID, "別人的", string(vo.MarketDataKindKCandle))

	require.NoError(t, repository.DeleteByUser(t.Context(), botRowOwnerID))

	_, findError := repository.FindOneByUser(t.Context(), botRowOwnerID)
	require.ErrorIs(t, findError, domains.ErrBinanceTradingKeyNotConfigured)
	assert.False(t, autoOrderOf(t, database, spotBot))
	assert.False(t, autoOrderOf(t, database, contractBot))
	assert.True(t, autoOrderOf(t, database, strangersBot))
}

func TestBinanceTradingKeyRepositoryDeleteByUserWithNothingStoredIsNotAFailure(t *testing.T) {
	database := newTradingKeyTestDatabase(t)

	require.NoError(t, persistence.NewBinanceTradingKeyRepository(database).DeleteByUser(t.Context(), botRowOwnerID))
}

func TestStrategyBotRepositoryEnableAutoOrderOnlyAgainstTheKeyThatWasChecked(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	tradingKeyRepository := persistence.NewBinanceTradingKeyRepository(database)
	strategyBotRepository := persistence.NewStrategyBotRepository(database)
	_, replaceError := tradingKeyRepository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, true), nil)
	require.NoError(t, replaceError)
	checkedKey, findError := tradingKeyRepository.FindOneByUser(t.Context(), botRowOwnerID)
	require.NoError(t, findError)
	savedBot, saveError := strategyBotRepository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)
	require.False(t, savedBot.AutoOrderEnabled, "a new bot starts switched off")

	require.NoError(t, strategyBotRepository.EnableAutoOrder(
		t.Context(), savedBot.ID, botRowOwnerID, checkedKey.UpdatedAt))
	assert.True(t, autoOrderOf(t, database, savedBot.ID))

	require.NoError(t, strategyBotRepository.DisableAutoOrder(t.Context(), savedBot.ID))
	assert.False(t, autoOrderOf(t, database, savedBot.ID))

	// The key was replaced after it was checked, so the check no longer stands.
	_, replaceAgainError := tradingKeyRepository.Replace(
		t.Context(), aTradingKeyOf(botRowOwnerID, "c3d4", true, false), nil)
	require.NoError(t, replaceAgainError)
	staleError := strategyBotRepository.EnableAutoOrder(
		t.Context(), savedBot.ID, botRowOwnerID, checkedKey.UpdatedAt)
	require.ErrorIs(t, staleError, domains.ErrStrategyBotAutoOrderKeyChanged)
	assert.False(t, autoOrderOf(t, database, savedBot.ID))

	require.NoError(t, tradingKeyRepository.DeleteByUser(t.Context(), botRowOwnerID))
	removedError := strategyBotRepository.EnableAutoOrder(
		t.Context(), savedBot.ID, botRowOwnerID, checkedKey.UpdatedAt)
	require.ErrorIs(t, removedError, domains.ErrStrategyBotAutoOrderKeyChanged)
	assert.False(t, autoOrderOf(t, database, savedBot.ID))
}

func TestStrategyBotRepositoryRewritesAndRoundsLeaveTheAutoOrderSwitchAlone(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	strategyBotRepository := persistence.NewStrategyBotRepository(database)
	botID := aSwitchedOnBot(t, database, botRowOwnerID, "早盤突破", string(vo.MarketDataKindKCandle))
	storedBot, findError := strategyBotRepository.FindOne(t.Context(), botID)
	require.NoError(t, findError)

	rewrite := storedBot
	rewrite.Name = "改名之後"
	rewrite.AutoOrderEnabled = false
	_, saveError := strategyBotRepository.Save(t.Context(), rewrite)
	require.NoError(t, saveError)
	require.NoError(t, strategyBotRepository.UpdateRunState(t.Context(), rewrite))

	assert.True(t, autoOrderOf(t, database, botID))
}

// A replacement in flight holds the key row; switching on must wait for it and then see that the key it checked is gone.
func TestStrategyBotRepositoryEnableAutoOrderWaitsOutAReplacementInFlight(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	tradingKeyRepository := persistence.NewBinanceTradingKeyRepository(database)
	_, replaceError := tradingKeyRepository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, true), nil)
	require.NoError(t, replaceError)
	checkedKey, findError := tradingKeyRepository.FindOneByUser(t.Context(), botRowOwnerID)
	require.NoError(t, findError)
	savedBot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	replacement := database.WithContext(t.Context()).Begin()
	require.NoError(t, replacement.Error)
	require.NoError(t, replacement.Model(&entities.BinanceTradingKey{}).
		Where(clause.Eq{Column: "user_id", Value: botRowOwnerID}).
		UpdateColumn("updated_at", checkedKey.UpdatedAt.Add(time.Second)).Error)

	enableFinished := make(chan error, 1)
	go func() {
		enableFinished <- persistence.NewStrategyBotRepository(database).EnableAutoOrder(
			t.Context(), savedBot.ID, botRowOwnerID, checkedKey.UpdatedAt)
	}()

	select {
	case <-enableFinished:
		t.Fatal("switching on did not wait for the replacement holding the key")
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, replacement.Commit().Error)
	require.ErrorIs(t, <-enableFinished, domains.ErrStrategyBotAutoOrderKeyChanged)
	assert.False(t, autoOrderOf(t, database, savedBot.ID))
}

func TestBinanceTradingKeyRepositoriesSaySoWhenStorageIsUnreachable(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	tradingKeyRepository := persistence.NewBinanceTradingKeyRepository(database)
	strategyBotRepository := persistence.NewStrategyBotRepository(database)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	_, findError := tradingKeyRepository.FindOneByUser(t.Context(), botRowOwnerID)
	_, replaceError := tradingKeyRepository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, false), nil)
	deleteError := tradingKeyRepository.DeleteByUser(t.Context(), botRowOwnerID)
	enableError := strategyBotRepository.EnableAutoOrder(t.Context(), 1, botRowOwnerID, time.Now())
	disableError := strategyBotRepository.DisableAutoOrder(t.Context(), 1)

	require.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrBinanceTradingKeyNotConfigured)
	require.Error(t, replaceError)
	require.Error(t, deleteError)
	require.Error(t, enableError)
	assert.NotErrorIs(t, enableError, domains.ErrStrategyBotAutoOrderKeyChanged)
	require.Error(t, disableError)
}

// Breaking the switch column makes the second half of each write fail, proving the key change and the switches commit together or not at all.
func TestBinanceTradingKeyRepositoryKeyChangesRollBackWhenSwitchesCannotBeWritten(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	tradingKeyRepository := persistence.NewBinanceTradingKeyRepository(database)
	_, replaceError := tradingKeyRepository.Replace(t.Context(), aTradingKeyOf(botRowOwnerID, "a1b2", true, true), nil)
	require.NoError(t, replaceError)
	checkedKey, findError := tradingKeyRepository.FindOneByUser(t.Context(), botRowOwnerID)
	require.NoError(t, findError)
	savedBot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)
	require.NoError(t, database.Migrator().RenameColumn(&entities.StrategyBot{}, "auto_order_enabled", "auto_order_enabled_moved"))
	t.Cleanup(func() {
		_ = database.Migrator().RenameColumn(&entities.StrategyBot{}, "auto_order_enabled_moved", "auto_order_enabled")
	})

	_, failedReplaceError := tradingKeyRepository.Replace(t.Context(),
		aTradingKeyOf(botRowOwnerID, "c3d4", true, false), []string{string(vo.MarketDataKindContractKCandle)})
	failedDeleteError := tradingKeyRepository.DeleteByUser(t.Context(), botRowOwnerID)
	failedEnableError := persistence.NewStrategyBotRepository(database).EnableAutoOrder(
		t.Context(), savedBot.ID, botRowOwnerID, checkedKey.UpdatedAt)

	require.Error(t, failedReplaceError)
	require.Error(t, failedDeleteError)
	require.Error(t, failedEnableError)
	assert.NotErrorIs(t, failedEnableError, domains.ErrStrategyBotAutoOrderKeyChanged)
	keptKey, keptError := tradingKeyRepository.FindOneByUser(t.Context(), botRowOwnerID)
	require.NoError(t, keptError)
	assert.Equal(t, "a1b2", keptKey.ApiKeyTail)
	assert.True(t, keptKey.ContractTradingEnabled)
}

// Making the key table unwritable fails the first half of each write, which must then leave every switch as it was.
func TestBinanceTradingKeyRepositoryLeavesSwitchesAloneWhenTheKeyCannotBeWritten(t *testing.T) {
	database := newTradingKeyTestDatabase(t)
	contractBot := aSwitchedOnBot(t, database, botRowOwnerID, "合約甲", string(vo.MarketDataKindContractKCandle))
	require.NoError(t, database.Migrator().RenameTable("BinanceTradingKeys", "BinanceTradingKeysMoved"))
	t.Cleanup(func() { _ = database.Migrator().RenameTable("BinanceTradingKeysMoved", "BinanceTradingKeys") })
	tradingKeyRepository := persistence.NewBinanceTradingKeyRepository(database)

	_, replaceError := tradingKeyRepository.Replace(t.Context(),
		aTradingKeyOf(botRowOwnerID, "c3d4", true, false), []string{string(vo.MarketDataKindContractKCandle)})
	deleteError := tradingKeyRepository.DeleteByUser(t.Context(), botRowOwnerID)
	enableError := persistence.NewStrategyBotRepository(database).EnableAutoOrder(
		t.Context(), contractBot, botRowOwnerID, time.Now())

	require.Error(t, replaceError)
	require.Error(t, deleteError)
	require.Error(t, enableError)
	assert.NotErrorIs(t, enableError, domains.ErrStrategyBotAutoOrderKeyChanged)
	assert.True(t, autoOrderOf(t, database, contractBot))
}
