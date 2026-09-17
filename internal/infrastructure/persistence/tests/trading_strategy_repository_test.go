package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// tradingStrategyRowOwnerID is whoever owns every trading strategy in this file
// unless a test says otherwise.
const tradingStrategyRowOwnerID = uint(1)

// newTradingStrategyTestDatabase is a cleared database with the person these belong
// to already in it. Planting them first is not scaffolding: the owner column carries
// a foreign key, so one owned by nobody is a row the schema refuses.
func newTradingStrategyTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: tradingStrategyRowOwnerID, Email: "rules-owner@example.com", PasswordProof: "a-proof",
	}).Error)

	return database
}

// aTradingStrategyRow is one set of rules with two sources and two condition trees:
//
//	buy:  ( A=buy and B=buy )
//	sell: A=sell
//
// The buy tree is nested so that the write path has to descend rather than write one
// flat level, which is the part of it worth proving.
func aTradingStrategyRow(name string) entities.TradingStrategy {
	return entities.TradingStrategy{
		OwnerID: tradingStrategyRowOwnerID, Name: name,
		SignalSources: []entities.TradingStrategySignalSource{
			{Label: "A", StrategyScriptID: 9, AggregationInterval: "1h",
				ParameterValues: []entities.TradingStrategySignalSourceParameterValue{
					{Name: "回看根數", Value: 20},
				}},
			{Label: "B", StrategyScriptID: 10, AggregationInterval: "5m"},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{Side: string(vo.TradingStrategyConditionSideBuy), Position: 0,
				Operator: string(vo.ConditionOperatorAnd),
				Children: []entities.TradingStrategyConditionNode{
					{Side: string(vo.TradingStrategyConditionSideBuy), Position: 0,
						SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
					{Side: string(vo.TradingStrategyConditionSideBuy), Position: 1,
						SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
				}},
			{Side: string(vo.TradingStrategyConditionSideSell), Position: 0,
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
	}
}

func TestTradingStrategyRepositorySaveAndReadBackAWholeOne(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	saved, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)
	require.NotZero(t, saved.ID)

	read, findError := repository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)

	assert.Equal(t, "黃金交叉", read.Name)
	require.Len(t, read.SignalSources, 2)
	require.Len(t, read.ConditionNodes, 4)

	// The nesting survives the round trip, which is the whole reason the write path
	// descends instead of handing a tree to the store and hoping.
	tradingStrategyDto := read.ToDto()
	assert.Equal(t, string(vo.ConditionOperatorAnd), tradingStrategyDto.BuyCondition.Operator)
	require.Len(t, tradingStrategyDto.BuyCondition.Conditions, 2)
	assert.Equal(t, "A", tradingStrategyDto.BuyCondition.Conditions[0].SourceLabel)
	assert.Equal(t, "B", tradingStrategyDto.BuyCondition.Conditions[1].SourceLabel)
	assert.Equal(t, "A", tradingStrategyDto.SellCondition.SourceLabel)
	assert.Equal(t, string(vo.SignalSell), tradingStrategyDto.SellCondition.Signal)

	// A source's parameter values come back with it, since nothing ever reads them
	// on their own.
	sourceValues := map[string][]float64{}
	for _, signalSource := range tradingStrategyDto.SignalSources {
		for _, parameterValue := range signalSource.ParameterValues {
			sourceValues[signalSource.Label] = append(
				sourceValues[signalSource.Label], parameterValue.Value)
		}
	}
	assert.Equal(t, []float64{20}, sourceValues["A"])
	assert.Empty(t, sourceValues["B"])
}

func TestTradingStrategyRepositorySaveReplacesTheSourcesAndTreesItHadBefore(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	saved, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	rewritten := aTradingStrategyRow("死亡交叉")
	rewritten.ID = saved.ID
	rewritten.SignalSources = []entities.TradingStrategySignalSource{
		{Label: "C", StrategyScriptID: 11, AggregationInterval: "1d"},
	}
	rewritten.ConditionNodes = []entities.TradingStrategyConditionNode{
		{Side: string(vo.TradingStrategyConditionSideBuy), Position: 0,
			SourceLabel: "C", ExpectedSignal: string(vo.SignalBuy)},
		{Side: string(vo.TradingStrategyConditionSideSell), Position: 0,
			SourceLabel: "C", ExpectedSignal: string(vo.SignalSell)},
	}

	rewrittenRow, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, saved.ID, rewrittenRow.ID)
	assert.Equal(t, "死亡交叉", rewrittenRow.Name)
	// Nothing of the old shape is left behind: a set of rules half rewritten could
	// name a label that no longer exists, and every bot following it would then run
	// that way every few minutes.
	require.Len(t, rewrittenRow.SignalSources, 1)
	assert.Equal(t, "C", rewrittenRow.SignalSources[0].Label)
	require.Len(t, rewrittenRow.ConditionNodes, 2)
}

// A rewrite must not be able to change hands or to forge a creation time, so it
// writes the name and nothing else about the row itself.
func TestTradingStrategyRepositorySaveLeavesTheOwnerAndCreationAlone(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	stranger := aSecondOwner(t, database)

	saved, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	rewritten := aTradingStrategyRow("黃金交叉")
	rewritten.ID = saved.ID
	rewritten.OwnerID = stranger

	rewrittenRow, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, tradingStrategyRowOwnerID, rewrittenRow.OwnerID)
	assert.Equal(t, saved.CreatedAt.UTC(), rewrittenRow.CreatedAt.UTC())
}

func TestTradingStrategyRepositorySaveRefusesANameThisPersonAlreadyUses(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	_, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	_, secondError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))

	require.ErrorIs(t, secondError, domains.ErrTradingStrategyNameConflict)
	assert.ErrorContains(t, secondError, "黃金交叉")
}

// Two people may each have one by the same name: a name is what its owner recognises
// it by, and nobody recognises a stranger's.
func TestTradingStrategyRepositoryLetsTwoPeopleUseOneName(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	stranger := aSecondOwner(t, database)

	_, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	strangersRow := aTradingStrategyRow("黃金交叉")
	strangersRow.OwnerID = stranger

	_, strangersError := repository.Save(t.Context(), strangersRow)
	require.NoError(t, strangersError)

	mine, listError := repository.FindAllByOwner(t.Context(), tradingStrategyRowOwnerID)
	require.NoError(t, listError)
	assert.Len(t, mine, 1)
}

func TestTradingStrategyRepositoryFindOneReportsTheDomainsNotFound(t *testing.T) {
	repository := persistence.NewTradingStrategyRepository(newTradingStrategyTestDatabase(t))

	_, findError := repository.FindOne(t.Context(), 4242)

	// Reported as the domain's own sentinel, so nobody outside has to recognise a
	// storage library's.
	require.ErrorIs(t, findError, domains.ErrTradingStrategyNotFound)
}

func TestTradingStrategyRepositoryDeleteTakesTheSourcesAndTreesWithIt(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	saved, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	require.NoError(t, repository.Delete(t.Context(), saved.ID))

	_, findError := repository.FindOne(t.Context(), saved.ID)
	require.ErrorIs(t, findError, domains.ErrTradingStrategyNotFound)

	// The cascade is declared rather than performed, so this is what proves no Go
	// code had to remember it.
	remainingSources := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.TradingStrategySignalSource{}).Count(&remainingSources).Error)
	assert.Zero(t, remainingSources)

	remainingNodes := int64(0)
	require.NoError(t, database.WithContext(t.Context()).
		Model(&entities.TradingStrategyConditionNode{}).Count(&remainingNodes).Error)
	assert.Zero(t, remainingNodes)
}

func TestTradingStrategyRepositoryListsByName(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	_, saveError := repository.Save(t.Context(), aTradingStrategyRow("死亡交叉"))
	require.NoError(t, saveError)
	_, saveError = repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	listed, listError := repository.FindAllByOwner(t.Context(), tradingStrategyRowOwnerID)
	require.NoError(t, listError)

	require.Len(t, listed, 2)
	assert.Equal(t, "死亡交叉", listed[0].Name)
	assert.Equal(t, "黃金交叉", listed[1].Name)
	// Everything a reader needs comes with each one, so a list is never N more reads.
	assert.Len(t, listed[0].SignalSources, 2)
	assert.Len(t, listed[0].ConditionNodes, 4)
}

func TestTradingStrategyRepositorySaysSoWhenStorageCannotAnswer(t *testing.T) {
	repository := persistence.NewTradingStrategyRepository(closedDatabase(t))

	// Every one of these must report the failure rather than quietly answering with
	// nothing — a list read as empty from a shut connection would tell somebody
	// their rules are gone.
	_, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	assert.Error(t, saveError)
	assert.NotErrorIs(t, saveError, domains.ErrTradingStrategyNameConflict)

	_, findError := repository.FindOne(t.Context(), 1)
	assert.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrTradingStrategyNotFound)

	_, listError := repository.FindAllByOwner(t.Context(), tradingStrategyRowOwnerID)
	assert.Error(t, listError)

	assert.Error(t, repository.Delete(t.Context(), 1))
}

func TestTradingStrategyRepositorySaveReportsAFailedRewriteAsItself(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	saved, saveError := repository.Save(t.Context(), aTradingStrategyRow("黃金交叉"))
	require.NoError(t, saveError)

	// A second one, so that renaming the first onto the second's name breaks the
	// index on the rewrite path rather than on the insert one.
	_, secondError := repository.Save(t.Context(), aTradingStrategyRow("死亡交叉"))
	require.NoError(t, secondError)

	rewritten := aTradingStrategyRow("死亡交叉")
	rewritten.ID = saved.ID

	_, rewriteError := repository.Save(t.Context(), rewritten)

	require.ErrorIs(t, rewriteError, domains.ErrTradingStrategyNameConflict)
}
