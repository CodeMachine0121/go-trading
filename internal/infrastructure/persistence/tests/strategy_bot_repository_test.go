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

// newStrategyBotTestDatabase is a cleared database with the person these bots belong
// to already in it. Planting them first is not scaffolding: the owner column carries
// a foreign key, so a bot owned by nobody is a row the schema refuses.
func newStrategyBotTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: botRowOwnerID, Email: "bot-owner@example.com", PasswordProof: "a-proof",
	}).Error)

	return database
}

// aBotRow is one bot with two sources and two condition trees:
//
//	buy:  ( A=buy and B=buy )
//	sell: A=sell
//
// The buy tree is nested so that the write path has to descend rather than write one
// flat level, which is the part of it worth proving.
func aBotRow(name string) entities.StrategyBot {
	return entities.StrategyBot{
		OwnerID: botRowOwnerID, Name: name, Symbol: "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotStopped),
		SignalSources: []entities.StrategyBotSignalSource{
			{Label: "A", StrategyID: 9, AggregationInterval: "1h",
				ParameterValues: []entities.StrategyBotSignalSourceParameterValue{
					{Name: "回看根數", Value: 20},
				}},
			{Label: "B", StrategyID: 10, AggregationInterval: "5m"},
		},
		ConditionNodes: []entities.StrategyBotConditionNode{
			{Side: string(vo.StrategyBotConditionSideBuy), Position: 0,
				Operator: string(vo.ConditionOperatorAnd),
				Children: []entities.StrategyBotConditionNode{
					{Side: string(vo.StrategyBotConditionSideBuy), Position: 0,
						SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
					{Side: string(vo.StrategyBotConditionSideBuy), Position: 1,
						SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
				}},
			{Side: string(vo.StrategyBotConditionSideSell), Position: 0,
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
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
	require.Len(t, readBot.SignalSources, 2)
	require.Len(t, readBot.ConditionNodes, 4)

	// The nesting survives the round trip, which is the whole reason the write path
	// descends instead of handing a tree to the store and hoping.
	botDto := readBot.ToDto()
	assert.Equal(t, string(vo.ConditionOperatorAnd), botDto.BuyCondition.Operator)
	require.Len(t, botDto.BuyCondition.Conditions, 2)
	assert.Equal(t, "A", botDto.BuyCondition.Conditions[0].SourceLabel)
	assert.Equal(t, "B", botDto.BuyCondition.Conditions[1].SourceLabel)
	assert.Equal(t, "A", botDto.SellCondition.SourceLabel)
	assert.Equal(t, string(vo.SignalSell), botDto.SellCondition.Signal)

	// A source's parameter values come back with it, since nothing ever reads them
	// on their own.
	sourceValues := map[string][]float64{}
	for _, signalSource := range botDto.SignalSources {
		for _, parameterValue := range signalSource.ParameterValues {
			sourceValues[signalSource.Label] = append(
				sourceValues[signalSource.Label], parameterValue.Value)
		}
	}
	assert.Equal(t, []float64{20}, sourceValues["A"])
	assert.Empty(t, sourceValues["B"])
}

func TestStrategyBotRepositorySaveReplacesTheSourcesAndTreesItHadBefore(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	rewritten := aBotRow("收盤反轉")
	rewritten.ID = savedBot.ID
	rewritten.SignalSources = []entities.StrategyBotSignalSource{
		{Label: "C", StrategyID: 11, AggregationInterval: "1d"},
	}
	rewritten.ConditionNodes = []entities.StrategyBotConditionNode{
		{Side: string(vo.StrategyBotConditionSideBuy), Position: 0,
			SourceLabel: "C", ExpectedSignal: string(vo.SignalBuy)},
		{Side: string(vo.StrategyBotConditionSideSell), Position: 0,
			SourceLabel: "C", ExpectedSignal: string(vo.SignalSell)},
	}

	rewrittenBot, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, savedBot.ID, rewrittenBot.ID)
	assert.Equal(t, "收盤反轉", rewrittenBot.Name)
	// Nothing of the old shape is left behind: a bot half rewritten could name a
	// label that no longer exists, and would then run that way every few minutes.
	require.Len(t, rewrittenBot.SignalSources, 1)
	assert.Equal(t, "C", rewrittenBot.SignalSources[0].Label)
	require.Len(t, rewrittenBot.ConditionNodes, 2)
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

func TestStrategyBotRepositoryDeleteTakesTheSourcesAndTreesWithIt(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	require.NoError(t, repository.Delete(t.Context(), savedBot.ID))

	_, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.ErrorIs(t, findError, domains.ErrStrategyBotNotFound)

	// The cascade is declared rather than performed, so this is what proves no Go
	// code had to remember it.
	remainingSources := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotSignalSource{}).Count(&remainingSources).Error)
	assert.Zero(t, remainingSources)

	remainingNodes := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.StrategyBotConditionNode{}).Count(&remainingNodes).Error)
	assert.Zero(t, remainingNodes)
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
	assert.Len(t, readBot.ConditionNodes, 4)
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
	// Everything a round needs comes with it, so a round is never a second read.
	assert.Len(t, dueBots[0].SignalSources, 2)
	assert.Len(t, dueBots[0].ConditionNodes, 4)
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
