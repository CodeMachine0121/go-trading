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
)

// botRowOwnerID is whoever owns every bot in this file unless a test says otherwise.
const botRowOwnerID = uint(1)

// botRowTradingStrategyID is the set of rules every bot in this file follows unless
// a test says otherwise.
const botRowTradingStrategyID = uint(1)

// newStrategyBotTestDatabase is a cleared database with the person these bots belong
// to, and the rules they follow, already in it. Planting them first is not
// scaffolding: both columns carry a foreign key, so a bot owned by nobody or
// following nothing is a row the schema refuses.
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

// aBotRow is one bot: a name, a market, how often, and the rules it follows. The
// rules themselves are not here — they are a thing of their own now, and a bot only
// names one.
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
	// The rules' current name comes back with the bot, so a list says what each bot
	// is doing without a second read per bot.
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
	// Pointing a bot at another set of rules is a rewrite like any other.
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

	// Reported as the domain's own sentinel, so nobody outside has to recognise a
	// storage library's.
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

	// The rules are a thing of their own, and other bots may follow them. Deleting
	// a machine must not take the rules with it.
	remainingTradingStrategies := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.TradingStrategy{}).Count(&remainingTradingStrategies).Error)
	assert.Equal(t, int64(1), remainingTradingStrategies)
}

// Both refusals that protect a set of rules — a rewrite blocked by a running bot, a
// delete blocked by any bot — are answered from this one read, whatever state those
// bots are in.
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
	// A round must not be able to rewrite a condition, so this is set to something
	// the write is expected to ignore entirely.
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

	// 「最後修改時間」是交給擁有者看的，就擺在建立時間旁邊。一台執行中的機器人
	// 每個觸發間隔都把它往前推一次、而沒有人改過任何東西的話，那一格就不再說得出
	// 任何事——一輪跑完不是一次修改。
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

	// Starting a bot clears all three. Written as a struct without naming the
	// columns, an empty value would read as "leave it alone" — and a bot would
	// start carrying yesterday's halt reason and never send its first signal.
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
	// Which rules it follows comes with it, so a round never has to ask twice which
	// bot it is about before it can ask what that bot does.
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

	// A system coming back after a long stop must not pull every bot it owns into
	// memory in order to run two of them.
	require.Len(t, dueBots, 2)
	// Oldest due first, so nothing starves behind a bot that wakes more often.
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
	// Two people may each have a bot by the same name: a name is what its owner
	// recognises a bot by, and nobody recognises a stranger's.
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

	// Every one of these must report the failure rather than quietly answering with
	// nothing — a scan that read "no bots are due" from a shut connection would
	// leave every running bot silent and nobody any the wiser.
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

	// A second bot, so that renaming the first onto the second's name breaks the
	// index on the rewrite path rather than on the insert one.
	_, secondError := repository.Save(t.Context(), aBotRow("收盤反轉"))
	require.NoError(t, secondError)

	rewritten := aBotRow("收盤反轉")
	rewritten.ID = savedBot.ID

	_, rewriteError := repository.Save(t.Context(), rewritten)

	require.ErrorIs(t, rewriteError, domains.ErrStrategyBotNameConflict)
}
