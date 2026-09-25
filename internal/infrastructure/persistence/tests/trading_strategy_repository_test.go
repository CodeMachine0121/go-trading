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

const tradingStrategyRowOwnerID = uint(1)

// newTradingStrategyTestDatabase seeds the owner that the foreign key requires.
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
// The nested buy tree forces the write path to recurse.
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

	tradingStrategyDto := read.ToDto()
	assert.Equal(t, string(vo.ConditionOperatorAnd), tradingStrategyDto.BuyCondition.Operator)
	require.Len(t, tradingStrategyDto.BuyCondition.Conditions, 2)
	assert.Equal(t, "A", tradingStrategyDto.BuyCondition.Conditions[0].SourceLabel)
	assert.Equal(t, "B", tradingStrategyDto.BuyCondition.Conditions[1].SourceLabel)
	assert.Equal(t, "A", tradingStrategyDto.SellCondition.SourceLabel)
	assert.Equal(t, string(vo.SignalSell), tradingStrategyDto.SellCondition.Signal)

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
	// Nothing of the old children remains after a rewrite.
	require.Len(t, rewrittenRow.SignalSources, 1)
	assert.Equal(t, "C", rewrittenRow.SignalSources[0].Label)
	require.Len(t, rewrittenRow.ConditionNodes, 2)
}

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

// The market data kind is fixed at creation because every source was validated against it.
func TestTradingStrategyRepositorySaveRewritesTheTradingModeButNeverTheKind(t *testing.T) {
	database := newTradingStrategyTestDatabase(t)
	repository := persistence.NewTradingStrategyRepository(database)

	contractRules := aTradingStrategyRow("合約黃金交叉")
	contractRules.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	contractRules.TradingMode = string(vo.ContractTradingModeLongShort)
	saved, saveError := repository.Save(t.Context(), contractRules)
	require.NoError(t, saveError)
	assert.Equal(t, "contractKCandle", saved.MarketDataKind)

	rewritten := aTradingStrategyRow("合約黃金交叉")
	rewritten.ID = saved.ID
	rewritten.MarketDataKind = string(vo.MarketDataKindKCandle)
	rewritten.TradingMode = string(vo.ContractTradingModeShortOnly)

	rewrittenRow, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	assert.Equal(t, "contractKCandle", rewrittenRow.MarketDataKind)
	assert.Equal(t, "shortOnly", rewrittenRow.TradingMode)
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

	// The cascade is declared in the schema, not performed in Go.
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
	assert.Len(t, listed[0].SignalSources, 2)
	assert.Len(t, listed[0].ConditionNodes, 4)
}

func TestTradingStrategyRepositorySaysSoWhenStorageCannotAnswer(t *testing.T) {
	repository := persistence.NewTradingStrategyRepository(closedDatabase(t))

	// A closed connection must fail loudly rather than read as an empty list.
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

	// A second strategy, so the rename breaks the name index on the rewrite path.
	_, secondError := repository.Save(t.Context(), aTradingStrategyRow("死亡交叉"))
	require.NoError(t, secondError)

	rewritten := aTradingStrategyRow("死亡交叉")
	rewritten.ID = saved.ID

	_, rewriteError := repository.Save(t.Context(), rewritten)

	require.ErrorIs(t, rewriteError, domains.ErrTradingStrategyNameConflict)
}
