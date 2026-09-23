package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aStoredContractStrategyScript is a strategy script that eats perpetual contract bars.
func aStoredContractStrategyScript(id uint, name string) entities.StrategyScript {
	strategyScript := aStoredStrategyScript(id, name)
	strategyScript.MarketDataKind = "contractKCandle"

	return strategyScript
}

func TestStrategyScriptApplicationCreateSettlesTheKindOfMarketTheScriptEats(t *testing.T) {
	testCases := []struct {
		name         string
		declared     string
		expectedKind string
	}{
		{name: "a script declared to eat contract bars eats contract bars", declared: "contractKCandle", expectedKind: "contractKCandle"},
		{name: "a script that declares nothing eats spot K candles", declared: "", expectedKind: "kCandle"},
		{name: "the spelling is forgiving about blanks and letter case", declared: " ContractKCandle ", expectedKind: "contractKCandle"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptApplicationUnderTest(t)
			fixture.strategyScriptRepository.EXPECT().
				Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
					assert.Equal(t, testCase.expectedKind, strategyScript.MarketDataKind)

					stored := aStoredStrategyScript(7, strategyScript.Name)
					stored.MarketDataKind = strategyScript.MarketDataKind

					return stored, nil
				})
			writeDto := aStrategyScriptWrite()
			writeDto.MarketDataKind = testCase.declared

			strategyScriptDto, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedKind, strategyScriptDto.MarketDataKind)
		})
	}
}

func TestStrategyScriptApplicationCreateRefusesAKindOfMarketItDoesNotKnow(t *testing.T) {
	// Nothing is stubbed on the repository: a refusal that still wrote would fail here.
	fixture := newStrategyScriptApplicationUnderTest(t)
	writeDto := aStrategyScriptWrite()
	writeDto.MarketDataKind = "選擇權"

	_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
	assert.Contains(t, err.Error(), "kCandle")
	assert.Contains(t, err.Error(), "contractKCandle")
}

func TestStrategyScriptApplicationUpdateKeepsTheKindOfMarketTheScriptWasCreatedWith(t *testing.T) {
	t.Run("switching a spot script to contract bars is refused and nothing is written", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.MarketDataKind = "contractKCandle"

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
		assert.Contains(t, err.Error(), "行情種類建立後不得更換")
	})

	t.Run("a rewrite that says nothing about the kind keeps contract bars", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredContractStrategyScript(7, "費率反轉"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				assert.Equal(t, "contractKCandle", strategyScript.MarketDataKind)
				assert.Equal(t, "費率反轉二號", strategyScript.Name)

				return aStoredContractStrategyScript(7, strategyScript.Name), nil
			})
		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.Name = "費率反轉二號"

		strategyScriptDto, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
		assert.Equal(t, "contractKCandle", strategyScriptDto.MarketDataKind)
	})

	t.Run("a rewrite that restates the same kind is not a change", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredContractStrategyScript(7, "費率反轉"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).Return(aStoredContractStrategyScript(7, "費率反轉二號"), nil)
		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.Name = "費率反轉二號"
		writeDto.MarketDataKind = "contractKCandle"

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
	})

	t.Run("a rewrite naming a kind nobody knows is refused as bad content", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.MarketDataKind = "選擇權"

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
	})
}

func TestStrategyScriptApplicationUpdateRefusesToGuessAStoredKindOfMarketItDoesNotKnow(t *testing.T) {
	// A stored kind nobody recognises is not read as the spot K candle: guessing
	// would let the rewrite quietly turn the script into something else.
	fixture := newStrategyScriptApplicationUnderTest(t)
	stored := aStoredStrategyScript(7, "二十根均線")
	stored.MarketDataKind = "選擇權"
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(stored, nil)
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 7

	_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.Error(t, err)
}

func TestStrategyScriptApplicationAvailableScriptsEachCarryTheirOwnKindOfMarket(t *testing.T) {
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), strategyScriptOwnerID).
		Return([]entities.StrategyScript{aStoredContractStrategyScript(1, "費率反轉")}, nil)
	adopted := aPublication(2, "別人的均線", 8)
	adopted.StrategyScript.MarketDataKind = "kCandle"
	fixture.strategyScriptRepository.EXPECT().
		FindAllAdoptedBy(gomock.Any(), strategyScriptOwnerID).
		Return([]entities.PublishedStrategyScript{adopted}, nil)

	availableStrategyScriptsDto, err := fixture.strategyScriptApplication.ListAvailableStrategyScripts(
		t.Context(), strategyScriptOwnerID)

	require.NoError(t, err)
	require.Len(t, availableStrategyScriptsDto.Mine, 1)
	require.Len(t, availableStrategyScriptsDto.Adopted, 1)
	assert.Equal(t, "contractKCandle", availableStrategyScriptsDto.Mine[0].MarketDataKind)
	assert.Equal(t, "kCandle", availableStrategyScriptsDto.Adopted[0].MarketDataKind)
}

func TestStrategyScriptMarketplaceShowsWhichKindOfMarketAPublishedScriptEats(t *testing.T) {
	fixture := newMarketplaceUnderTest(t)
	publication := aPublication(2, "別人的費率反轉", 8)
	publication.StrategyScript.MarketDataKind = "contractKCandle"
	fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
		Return([]entities.PublishedStrategyScript{publication}, nil)

	publishedStrategyScriptDtos, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

	require.NoError(t, err)
	require.Len(t, publishedStrategyScriptDtos, 1)
	assert.Equal(t, "contractKCandle", publishedStrategyScriptDtos[0].MarketDataKind)
}
