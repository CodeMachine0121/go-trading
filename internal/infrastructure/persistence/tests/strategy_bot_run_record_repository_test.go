package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var runRecordRanAt = time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

// aBotToRecordAgainst plants a bot for the history to hang off. The column carries a
// foreign key, so a history belonging to nobody is a row the schema refuses.
func aBotToRecordAgainst(t *testing.T, database *gorm.DB) uint {
	repository := persistence.NewStrategyBotRepository(database)
	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	return savedBot.ID
}

func TestStrategyBotRunRecordRepositoryNumbersEachRoundInTurn(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), botID, runRecordRanAt, "hold"))
	require.NoError(t, repository.Append(t.Context(), botID, runRecordRanAt, "buy"))
	require.NoError(t, repository.Append(t.Context(), botID, runRecordRanAt, "sell"))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)

	// 最新的排前面：打開歷史的人問的是「最近怎麼了」。
	require.Len(t, runRecords, 3)
	assert.Equal(t, 3, runRecords[0].RunNumber)
	assert.Equal(t, "sell", runRecords[0].Result)
	assert.Equal(t, 1, runRecords[2].RunNumber)
}

func TestStrategyBotRunRecordRepositoryKeepsOnlyTheLastFifty(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	// 一台五分鐘的機器人一天跑 288 輪。什麼都留的話，那張表只會長不會縮。
	for round := 0; round < 55; round++ {
		require.NoError(t, repository.Append(t.Context(), botID, runRecordRanAt, "hold"))
	}

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)

	assert.Len(t, runRecords, 50)

	// 問的是**表裡真的剩幾列**，不是讀出來幾列。只看讀出來的話，
	// 一個把 Limit 訂在 50、卻從來沒刪過任何東西的實作也一樣是綠的——
	// 而那正是這一條要防的：一張只會長不會縮的表。
	stored := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotRunRecord{}).Count(&stored).Error)
	assert.Equal(t, int64(50), stored)
	// 編號**不重排**：Run 55 在 Run 1 被清掉之後還是 Run 55。
	// 重排的話，同一輪會因為別人什麼時候來看而有兩個名字。
	assert.Equal(t, 55, runRecords[0].RunNumber)
	assert.Equal(t, 6, runRecords[49].RunNumber)
}

func TestStrategyBotRunRecordRepositoryKeepsEachBotsHistoryToItself(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	firstBotID := aBotToRecordAgainst(t, database)

	botRepository := persistence.NewStrategyBotRepository(database)
	secondBot, saveError := botRepository.Save(t.Context(), aBotRow("收盤反轉"))
	require.NoError(t, saveError)

	require.NoError(t, repository.Append(t.Context(), firstBotID, runRecordRanAt, "buy"))
	require.NoError(t, repository.Append(t.Context(), secondBot.ID, runRecordRanAt, "sell"))

	// 每一台各自從 Run 1 開始數，而且只看得到自己的。
	firstHistory, findError := repository.FindLatestByBot(t.Context(), firstBotID)
	require.NoError(t, findError)
	require.Len(t, firstHistory, 1)
	assert.Equal(t, 1, firstHistory[0].RunNumber)
	assert.Equal(t, "buy", firstHistory[0].Result)

	secondHistory, findError := repository.FindLatestByBot(t.Context(), secondBot.ID)
	require.NoError(t, findError)
	require.Len(t, secondHistory, 1)
	assert.Equal(t, 1, secondHistory[0].RunNumber)
	assert.Equal(t, "sell", secondHistory[0].Result)
}

func TestStrategyBotRunRecordRepositoryLosesTheHistoryWithTheBot(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), botID, runRecordRanAt, "buy"))
	require.NoError(t, persistence.NewStrategyBotRepository(database).Delete(t.Context(), botID))

	// cascade 是宣告出來的，不是哪一段 Go 記得要做的——這一條就是那個保證。
	remaining := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotRunRecord{}).Count(&remaining).Error)
	assert.Zero(t, remaining)
}

func TestStrategyBotRunRecordRepositorySaysSoWhenStorageCannotAnswer(t *testing.T) {
	repository := persistence.NewStrategyBotRunRecordRepository(closedDatabase(t))

	assert.Error(t, repository.Append(t.Context(), 1, runRecordRanAt, "buy"))

	_, findError := repository.FindLatestByBot(t.Context(), 1)
	assert.Error(t, findError)
}

func TestStrategyBotRunRecordRepositoryHandsOutAnEmptyHistoryRatherThanAFailure(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)

	runRecords, findError := persistence.NewStrategyBotRunRecordRepository(database).
		FindLatestByBot(t.Context(), botID)

	// 一台剛建好、還沒跑過的機器人是正常狀態，不是錯誤。
	require.NoError(t, findError)
	assert.Empty(t, runRecords)
	assert.NotNil(t, runRecords)
}
