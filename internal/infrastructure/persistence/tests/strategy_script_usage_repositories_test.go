package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyScriptRepositoryStoresAScriptWithNoUsageAsACalculationScript(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	saved, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("均線"))
	require.NoError(t, saveError)

	found, findError := strategyScriptRepository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "calculation", found.Usage)
}

func TestStrategyScriptRepositoryUpdateUsageWritesTheUsageAlone(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	saved, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("均線"))
	require.NoError(t, saveError)

	switched, switchError := strategyScriptRepository.UpdateUsage(t.Context(), saved.ID, "chart")

	require.NoError(t, switchError)
	assert.Equal(t, "chart", switched.Usage)
	assert.Equal(t, saved.Script, switched.Script)
	assert.Equal(t, "均線", switched.Name)
}

func TestStrategyScriptRepositoryRewriteLeavesTheUsageAlone(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	saved, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("均線"))
	require.NoError(t, saveError)
	_, switchError := strategyScriptRepository.UpdateUsage(t.Context(), saved.ID, "chart")
	require.NoError(t, switchError)

	rewrite := strategyScriptNamed("均線 v2")
	rewrite.ID = saved.ID
	rewrite.Usage = "calculation"
	rewritten, rewriteError := strategyScriptRepository.Update(t.Context(), rewrite)

	require.NoError(t, rewriteError)
	assert.Equal(t, "chart", rewritten.Usage)
}

func TestStrategyScriptRepositoryUpdateUsageReportsNotFound(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	_, switchError := strategyScriptRepository.UpdateUsage(t.Context(), 404, "chart")

	require.ErrorIs(t, switchError, domains.ErrStrategyScriptNotFound)
}

func TestTradingStrategyRepositoryNamesTheStrategiesComputingWithAScript(t *testing.T) {
	tradingStrategyRepository := persistence.NewTradingStrategyRepository(newTradingStrategyTestDatabase(t))
	for _, name := range []string{"乙策略", "甲策略"} {
		_, saveError := tradingStrategyRepository.Save(t.Context(), aTradingStrategyRow(name))
		require.NoError(t, saveError)
	}

	testCases := []struct {
		name             string
		strategyScriptID uint
		expectedNames    []string
	}{
		{name: "every strategy naming it, by name", strategyScriptID: 9, expectedNames: []string{"乙策略", "甲策略"}},
		{name: "nobody names it", strategyScriptID: 404, expectedNames: []string{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			names, findError := tradingStrategyRepository.FindNamesByStrategyScript(t.Context(), testCase.strategyScriptID)

			require.NoError(t, findError)
			assert.Equal(t, testCase.expectedNames, names)
		})
	}
}

func TestSymbolScreeningProfileRepositoryNamesTheProfilesScreeningWithAScript(t *testing.T) {
	profileRepository := persistence.NewSymbolScreeningProfileRepository(newScreeningTestDatabase(t))
	for _, name := range []string{"新幣", "大型幣"} {
		_, saveError := profileRepository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, name))
		require.NoError(t, saveError)
	}

	testCases := []struct {
		name             string
		strategyScriptID uint
		expectedNames    []string
	}{
		{name: "every profile screening with it, by name", strategyScriptID: 108, expectedNames: []string{"大型幣", "新幣"}},
		{name: "nobody screens with it", strategyScriptID: 404, expectedNames: []string{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			names, findError := profileRepository.FindNamesByStrategyScript(t.Context(), testCase.strategyScriptID)

			require.NoError(t, findError)
			assert.Equal(t, testCase.expectedNames, names)
		})
	}
}

func TestUsageLookupsSaySoWhenTheStoreIsGone(t *testing.T) {
	database := closedDatabase(t)

	_, switchError := persistence.NewStrategyScriptRepository(database).UpdateUsage(t.Context(), 1, "chart")
	assert.Error(t, switchError)
	assert.NotErrorIs(t, switchError, domains.ErrStrategyScriptNotFound)

	_, tradingStrategyError := persistence.NewTradingStrategyRepository(database).FindNamesByStrategyScript(t.Context(), 1)
	assert.Error(t, tradingStrategyError)

	_, profileError := persistence.NewSymbolScreeningProfileRepository(database).FindNamesByStrategyScript(t.Context(), 1)
	assert.Error(t, profileError)
}
