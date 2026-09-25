package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var runRecordRanAt = time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

// aBotToRecordAgainst plants the bot the history's foreign key requires.
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

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "hold"}))
	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "buy"}))
	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "sell"}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)

	require.Len(t, runRecords, 3)
	assert.Equal(t, 3, runRecords[0].RunNumber)
	assert.Equal(t, "sell", runRecords[0].Result)
	assert.Equal(t, 1, runRecords[2].RunNumber)
}

func TestStrategyBotRunRecordRepositoryKeepsOnlyTheLastFifty(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	// 保留筆數有上限，否則一天 288 輪的表只會長不會縮。
	for round := 0; round < 55; round++ {
		require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
			StrategyBotID: botID, RanAt: runRecordRanAt, Result: "hold"}))
	}

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)

	assert.Len(t, runRecords, 50)

	// 驗證表裡實際剩下的列數，而不只是讀出來的列數。
	stored := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotRunRecord{}).Count(&stored).Error)
	assert.Equal(t, int64(50), stored)
	// 編號不重排：Run 1 被清掉後 Run 55 仍是 Run 55。
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

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: firstBotID, RanAt: runRecordRanAt, Result: "buy"}))
	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: secondBot.ID, RanAt: runRecordRanAt, Result: "sell"}))

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

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "buy"}))
	require.NoError(t, persistence.NewStrategyBotRepository(database).Delete(t.Context(), botID))

	// 由 schema 宣告的 cascade 刪除，而非 Go 程式碼。
	remaining := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotRunRecord{}).Count(&remaining).Error)
	assert.Zero(t, remaining)
}

func TestStrategyBotRunRecordRepositorySaysSoWhenStorageCannotAnswer(t *testing.T) {
	repository := persistence.NewStrategyBotRunRecordRepository(closedDatabase(t))

	assert.Error(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: 1, RanAt: runRecordRanAt, Result: "buy"}))

	_, findError := repository.FindLatestByBot(t.Context(), 1)
	assert.Error(t, findError)
}

func TestStrategyBotRunRecordRepositoryHandsOutAnEmptyHistoryRatherThanAFailure(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)

	runRecords, findError := persistence.NewStrategyBotRunRecordRepository(database).
		FindLatestByBot(t.Context(), botID)

	require.NoError(t, findError)
	assert.Empty(t, runRecords)
	assert.NotNil(t, runRecords)
}

// Stored suggestion figures must be what the round suggested, not what current settings would produce.
func TestStrategyBotRunRecordRepositoryAppendRemembersWhatTheRoundSuggested(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "sell",
		HasPositionPlan: true,
		PositionPlan: dto.PositionPlanDto{
			Stake:           decimal.NewFromInt(5000),
			Affordable:      true,
			StopLossPrice:   decimal.RequireFromString("66105.915"),
			HasStopLoss:     true,
			TakeProfitPrice: decimal.RequireFromString("60971.475"),
			HasTakeProfit:   true,
		},
	}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)

	assert.True(t, runRecords[0].SuggestedStake.Valid)
	assert.Equal(t, "5000", runRecords[0].SuggestedStake.Decimal.String())
	assert.Equal(t, "66105.915", runRecords[0].SuggestedStopLossPrice.Decimal.String())
	assert.Equal(t, "60971.475", runRecords[0].SuggestedTakeProfitPrice.Decimal.String())
}

// No suggestion is stored as nil, not zero, since a zero stop-loss price is a real figure.
func TestStrategyBotRunRecordRepositoryAppendRemembersNothingWhenNothingWasSuggested(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "hold"}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)

	assert.False(t, runRecords[0].SuggestedStake.Valid)
	assert.False(t, runRecords[0].SuggestedStopLossPrice.Valid)
	assert.False(t, runRecords[0].SuggestedTakeProfitPrice.Valid)
	runRecordDto := runRecords[0].ToDto()
	assert.Nil(t, runRecordDto.SuggestedStake)
	assert.Nil(t, runRecordDto.SuggestedStopLossPrice)
	assert.Nil(t, runRecordDto.SuggestedTakeProfitPrice)
}

// An unaffordable stake is stored as no suggestion.
func TestStrategyBotRunRecordRepositoryAppendRemembersNothingForAnUnaffordableStake(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "buy",
		HasPositionPlan: true,
		PositionPlan: dto.PositionPlanDto{
			Stake: decimal.NewFromInt(8000), Affordable: false},
	}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)
	assert.False(t, runRecords[0].SuggestedStake.Valid)
}
