package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const botRowOwnerID = uint(1)

const botRowTradingStrategyID = uint(1)

// newStrategyBotTestDatabase seeds the owner and trading strategy that the bot foreign keys require.
func newStrategyBotTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: botRowOwnerID, Email: "bot-owner@example.com", PasswordProof: "a-proof",
	}).Error)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.TradingStrategy{
		ID: botRowTradingStrategyID, OwnerID: botRowOwnerID, Name: "黃金交叉",
	}).Error)

	return database
}

func aBotRow(name string) entities.StrategyBot {
	return entities.StrategyBot{
		OwnerID: botRowOwnerID, Name: name, Symbol: "BTCUSDT",
		TradingStrategyID:      botRowTradingStrategyID,
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotStopped),
	}
}

func TestStrategyBotRepositorySaveAndReadBackAWholeBot(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)
	require.NotZero(t, savedBot.ID)

	readBot, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.NoError(t, findError)

	assert.Equal(t, "早盤突破", readBot.Name)
	assert.Equal(t, "BTCUSDT", readBot.Symbol)
	assert.Equal(t, botRowTradingStrategyID, readBot.TradingStrategyID)
	// The trading strategy is preloaded so a listing needs no read per bot.
	assert.Equal(t, "黃金交叉", readBot.TradingStrategy.Name)
}

func TestStrategyBotRepositorySaveReplacesWhatItHadBefore(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	require.NoError(t, database.WithContext(t.Context()).Create(&entities.TradingStrategy{
		ID: 2, OwnerID: botRowOwnerID, Name: "死亡交叉",
	}).Error)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	rewritten := aBotRow("收盤反轉")
	rewritten.ID = savedBot.ID
	rewritten.Symbol = "ETHUSDT"
	rewritten.TradingStrategyID = 2
	rewritten.TriggerIntervalMinutes = 15

	rewrittenBot, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, savedBot.ID, rewrittenBot.ID)
	assert.Equal(t, "收盤反轉", rewrittenBot.Name)
	assert.Equal(t, "ETHUSDT", rewrittenBot.Symbol)
	assert.Equal(t, uint(2), rewrittenBot.TradingStrategyID)
	assert.Equal(t, 15, rewrittenBot.TriggerIntervalMinutes)
}

func TestStrategyBotRepositorySaveRefusesANameThisPersonAlreadyUses(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	_, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	_, secondError := repository.Save(t.Context(), aBotRow("早盤突破"))

	require.ErrorIs(t, secondError, domains.ErrStrategyBotNameConflict)
	assert.ErrorContains(t, secondError, "早盤突破")
}

func TestStrategyBotRepositoryFindOneReportsTheDomainsNotFound(t *testing.T) {
	repository := persistence.NewStrategyBotRepository(newStrategyBotTestDatabase(t))

	_, findError := repository.FindOne(t.Context(), 4242)

	require.ErrorIs(t, findError, domains.ErrStrategyBotNotFound)
}

func TestStrategyBotRepositoryDeleteLeavesTheRulesItFollowedAlone(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	require.NoError(t, repository.Delete(t.Context(), savedBot.ID))

	_, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.ErrorIs(t, findError, domains.ErrStrategyBotNotFound)

	// Deleting a bot must not delete the trading strategy, which other bots may follow.
	remainingTradingStrategies := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.TradingStrategy{}).Count(&remainingTradingStrategies).Error)
	assert.Equal(t, int64(1), remainingTradingStrategies)
}

func TestStrategyBotRepositoryFindAllByTradingStrategyAnswersRunningAndStoppedAlike(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	require.NoError(t, database.WithContext(t.Context()).Create(&entities.TradingStrategy{
		ID: 2, OwnerID: botRowOwnerID, Name: "死亡交叉",
	}).Error)

	runningBot, saveError := repository.Save(t.Context(), aBotRow("A 執行中的"))
	require.NoError(t, saveError)
	runningBot.RunState = string(vo.StrategyBotRunning)
	require.NoError(t, repository.UpdateRunState(t.Context(), runningBot))

	_, saveError = repository.Save(t.Context(), aBotRow("B 已停止的"))
	require.NoError(t, saveError)

	otherRulesBot := aBotRow("跟別份規則的")
	otherRulesBot.TradingStrategyID = 2
	_, saveError = repository.Save(t.Context(), otherRulesBot)
	require.NoError(t, saveError)

	followers, findError := repository.FindAllByTradingStrategy(
		t.Context(), botRowTradingStrategyID)
	require.NoError(t, findError)

	require.Len(t, followers, 2)
	assert.Equal(t, "A 執行中的", followers[0].Name)
	assert.Equal(t, "B 已停止的", followers[1].Name)
}

func TestStrategyBotRepositoryFindAllByStrategyScriptReachesTheScriptThroughTheTradingStrategy(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	for _, strategyScript := range []entities.StrategyScript{
		{ID: 11, OwnerID: botRowOwnerID, Name: "均線", Script: "x", ResultType: "signal"},
		{ID: 12, OwnerID: botRowOwnerID, Name: "量能", Script: "x", ResultType: "signal"},
	} {
		require.NoError(t, database.WithContext(t.Context()).Create(&strategyScript).Error)
	}
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.TradingStrategy{
		ID: 2, OwnerID: botRowOwnerID, Name: "死亡交叉",
	}).Error)
	for _, signalSource := range []entities.TradingStrategySignalSource{
		{TradingStrategyID: botRowTradingStrategyID, Label: "A", StrategyScriptID: 11, AggregationInterval: "1h"},
		{TradingStrategyID: 2, Label: "A", StrategyScriptID: 12, AggregationInterval: "1h"},
	} {
		require.NoError(t, database.WithContext(t.Context()).Create(&signalSource).Error)
	}

	_, saveError := repository.Save(t.Context(), aBotRow("用均線的"))
	require.NoError(t, saveError)
	otherRulesBot := aBotRow("用量能的")
	otherRulesBot.TradingStrategyID = 2
	_, saveError = repository.Save(t.Context(), otherRulesBot)
	require.NoError(t, saveError)

	botsUsingScript, findError := repository.FindAllByStrategyScript(t.Context(), 11)
	require.NoError(t, findError)

	require.Len(t, botsUsingScript, 1)
	assert.Equal(t, "用均線的", botsUsingScript[0].Name)
}

func TestStrategyBotRepositoryUpdateRunStateTouchesOnlyABotsLife(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	dueAt := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
	savedBot.RunState = string(vo.StrategyBotRunning)
	savedBot.NextRunAt = dueAt
	savedBot.LastSentSignal = string(vo.SignalBuy)
	savedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)
	savedBot.Conflicting = true
	// A round must not rewrite configuration, so this change must be ignored.
	savedBot.Name = "這個名字不該被寫進去"

	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))

	readBot, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.NoError(t, findError)

	assert.Equal(t, string(vo.StrategyBotRunning), readBot.RunState)
	assert.Equal(t, dueAt, readBot.NextRunAt.UTC())
	assert.Equal(t, string(vo.SignalBuy), readBot.LastSentSignal)
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), readBot.HaltReason)
	assert.True(t, readBot.Conflicting)
	assert.Equal(t, "早盤突破", readBot.Name)
}

func TestStrategyBotRepositoryUpdateRunStateLeavesTheLastModifiedTimeAlone(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	savedBot.RunState = string(vo.StrategyBotRunning)
	savedBot.NextRunAt = time.Now().UTC()
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))

	readBot, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.NoError(t, findError)

	// 最後修改時間是給擁有者看的，一輪跑完不算修改，不應推進它。
	assert.Equal(t, savedBot.UpdatedAt.UTC(), readBot.UpdatedAt.UTC())
	assert.Equal(t, savedBot.CreatedAt.UTC(), readBot.CreatedAt.UTC())
}

func TestStrategyBotRepositoryUpdateRunStateClearsRatherThanSkippingEmptyValues(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	savedBot.LastSentSignal = string(vo.SignalBuy)
	savedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)
	savedBot.Conflicting = true
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))

	// Columns are named so these empty values are written rather than skipped.
	savedBot.LastSentSignal = ""
	savedBot.HaltReason = ""
	savedBot.Conflicting = false
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))

	readBot, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.NoError(t, findError)
	assert.Empty(t, readBot.LastSentSignal)
	assert.Empty(t, readBot.HaltReason)
	assert.False(t, readBot.Conflicting)
}

func TestStrategyBotRepositoryFindDueAnswersOnlyRunningBotsThatAreDue(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	dueBot, saveError := repository.Save(t.Context(), aBotRow("到期的"))
	require.NoError(t, saveError)
	dueBot.RunState = string(vo.StrategyBotRunning)
	dueBot.NextRunAt = now.Add(-time.Minute)
	require.NoError(t, repository.UpdateRunState(t.Context(), dueBot))

	notYetBot, saveError := repository.Save(t.Context(), aBotRow("還沒到期的"))
	require.NoError(t, saveError)
	notYetBot.RunState = string(vo.StrategyBotRunning)
	notYetBot.NextRunAt = now.Add(time.Hour)
	require.NoError(t, repository.UpdateRunState(t.Context(), notYetBot))

	stoppedBot, saveError := repository.Save(t.Context(), aBotRow("已停止的"))
	require.NoError(t, saveError)
	stoppedBot.NextRunAt = now.Add(-time.Hour)
	require.NoError(t, repository.UpdateRunState(t.Context(), stoppedBot))

	dueBots, findError := repository.FindDue(t.Context(), now, 10)
	require.NoError(t, findError)

	require.Len(t, dueBots, 1)
	assert.Equal(t, "到期的", dueBots[0].Name)
	assert.Equal(t, botRowTradingStrategyID, dueBots[0].TradingStrategyID)
}

func TestStrategyBotRepositoryFindDueHonoursTheCapOnTheReadItself(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	for index, name := range []string{"第一台", "第二台", "第三台"} {
		bot, saveError := repository.Save(t.Context(), aBotRow(name))
		require.NoError(t, saveError)
		bot.RunState = string(vo.StrategyBotRunning)
		bot.NextRunAt = now.Add(-time.Duration(3-index) * time.Minute)
		require.NoError(t, repository.UpdateRunState(t.Context(), bot))
	}

	dueBots, findError := repository.FindDue(t.Context(), now, 2)
	require.NoError(t, findError)

	// The due query is limited so a long outage does not load every bot.
	require.Len(t, dueBots, 2)
	assert.Equal(t, "第一台", dueBots[0].Name)
	assert.Equal(t, "第二台", dueBots[1].Name)
}

func TestStrategyBotRepositoryCountsAndListsPerOwner(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	stranger := aSecondOwner(t, database)

	mine, saveError := repository.Save(t.Context(), aBotRow("我的"))
	require.NoError(t, saveError)
	mine.RunState = string(vo.StrategyBotRunning)
	require.NoError(t, repository.UpdateRunState(t.Context(), mine))

	strangersBot := aBotRow("我的")
	strangersBot.OwnerID = stranger
	strangersSaved, saveError := repository.Save(t.Context(), strangersBot)
	// Bot names are unique per owner, not globally.
	require.NoError(t, saveError)
	strangersSaved.RunState = string(vo.StrategyBotRunning)
	require.NoError(t, repository.UpdateRunState(t.Context(), strangersSaved))

	myBots, listError := repository.FindAllByOwner(t.Context(), botRowOwnerID)
	require.NoError(t, listError)
	require.Len(t, myBots, 1)
	assert.Equal(t, mine.ID, myBots[0].ID)

	runningCount, countError := repository.CountRunningByOwner(t.Context(), botRowOwnerID)
	require.NoError(t, countError)
	assert.Equal(t, 1, runningCount)
}

func TestStrategyBotRepositorySaysSoWhenStorageCannotAnswer(t *testing.T) {
	repository := persistence.NewStrategyBotRepository(closedDatabase(t))

	// A closed connection must fail loudly, not read as "no bots are due".
	_, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	assert.Error(t, saveError)
	assert.NotErrorIs(t, saveError, domains.ErrStrategyBotNameConflict)

	_, findError := repository.FindOne(t.Context(), 1)
	assert.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrStrategyBotNotFound)

	_, listError := repository.FindAllByOwner(t.Context(), botRowOwnerID)
	assert.Error(t, listError)

	assert.Error(t, repository.Delete(t.Context(), 1))

	assert.Error(t, repository.UpdateRunState(t.Context(), aBotRow("早盤突破")))

	_, countError := repository.CountRunningByOwner(t.Context(), botRowOwnerID)
	assert.Error(t, countError)

	_, dueError := repository.FindDue(t.Context(), time.Now(), 10)
	assert.Error(t, dueError)

	_, followersError := repository.FindAllByTradingStrategy(t.Context(), 1)
	assert.Error(t, followersError)
}

func TestStrategyBotRepositorySaveReportsAFailedRewriteAsItself(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	// A second bot, so the rename breaks the name index on the rewrite path.
	_, secondError := repository.Save(t.Context(), aBotRow("收盤反轉"))
	require.NoError(t, secondError)

	rewritten := aBotRow("收盤反轉")
	rewritten.ID = savedBot.ID

	_, rewriteError := repository.Save(t.Context(), rewritten)

	require.ErrorIs(t, rewriteError, domains.ErrStrategyBotNameConflict)
}

func TestStrategyBotRepositorySaveKeepsAndRewritesThePositionPlan(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	plannedBot := aBotRow("早盤突破")
	plannedBot.PositionPlanCapital = decimal.NewFromInt(50000)
	plannedBot.PositionPlanSizingMode = string(vo.PositionSizingModePercentage)
	plannedBot.PositionPlanSizingValue = decimal.NewFromInt(10)
	plannedBot.PositionPlanStopLossPercentage = decimal.NewFromInt(3)
	plannedBot.PositionPlanTakeProfitPercentage = decimal.NewFromInt(5)

	saved, saveError := repository.Save(t.Context(), plannedBot)
	require.NoError(t, saveError)

	readBack, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "50000", readBack.PositionPlanCapital.String())
	assert.Equal(t, "10", readBack.PositionPlanSizingValue.String())
	assert.Equal(t, "3", readBack.PositionPlanStopLossPercentage.String())
	assert.Equal(t, "5", readBack.PositionPlanTakeProfitPercentage.String())

	rewritten := aBotRow("早盤突破")
	rewritten.ID = saved.ID
	rewritten.PositionPlanCapital = decimal.NewFromInt(80000)
	rewritten.PositionPlanSizingMode = string(vo.PositionSizingModeFixedAmount)
	rewritten.PositionPlanSizingValue = decimal.NewFromInt(8000)
	rewritten.PositionPlanStopLossPercentage = decimal.NewFromInt(2)

	_, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	rewrittenRow, findRewrittenError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findRewrittenError)
	assert.Equal(t, "80000", rewrittenRow.PositionPlanCapital.String())
	assert.Equal(t,
		string(vo.PositionSizingModeFixedAmount), rewrittenRow.PositionPlanSizingMode)
	assert.Equal(t, "8000", rewrittenRow.PositionPlanSizingValue.String())
	assert.Equal(t, "2", rewrittenRow.PositionPlanStopLossPercentage.String())
	assert.True(t, rewrittenRow.PositionPlanTakeProfitPercentage.IsZero())
}

// A row with no plan (as stored before these columns existed) must read back as no suggestion.
func TestStrategyBotRepositorySaveLeavesABotWithoutAPositionPlanSuggestingNothing(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	saved, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	readBack, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.True(t, readBack.PositionPlanCapital.IsZero())
}
