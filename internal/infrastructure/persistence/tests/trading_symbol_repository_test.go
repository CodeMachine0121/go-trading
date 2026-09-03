package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registeredNames(t *testing.T, repository *persistence.TradingSymbolRepository) []string {
	t.Helper()

	tradingSymbols, findError := repository.FindAll(t.Context())
	require.NoError(t, findError)

	names := make([]string, 0, len(tradingSymbols))
	for _, tradingSymbol := range tradingSymbols {
		names = append(names, tradingSymbol.Symbol)
	}

	return names
}

func TestRegisterAll(t *testing.T) {
	t.Run("stores the given symbols and hands them back by name", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))

		require.NoError(t, repository.RegisterAll(t.Context(), []entities.TradingSymbol{
			{Symbol: "SOLUSDT"}, {Symbol: "BTCUSDT"},
		}))

		assert.Equal(t, []string{"BTCUSDT", "SOLUSDT"}, registeredNames(t, repository))
	})

	t.Run("registering the same symbol again changes nothing and is not an error", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		require.NoError(t, repository.RegisterAll(t.Context(), []entities.TradingSymbol{{Symbol: "BTCUSDT"}}))

		require.NoError(t, repository.RegisterAll(t.Context(), []entities.TradingSymbol{
			{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"},
		}))

		assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, registeredNames(t, repository))
	})

	t.Run("registering nothing is not an error", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))

		require.NoError(t, repository.RegisterAll(t.Context(), []entities.TradingSymbol{}))

		assert.Empty(t, registeredNames(t, repository))
	})

	t.Run("nothing registered is an empty list", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))

		assert.Empty(t, registeredNames(t, repository))
	})
}

func TestTradingSymbolStorageFailures(t *testing.T) {
	t.Run("reports a failure reading the registered markets", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(closedDatabase(t))

		_, findError := repository.FindAll(t.Context())

		assert.Error(t, findError)
	})

	t.Run("reports a failure while registering", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(closedDatabase(t))

		registerError := repository.RegisterAll(t.Context(), []entities.TradingSymbol{{Symbol: "BTCUSDT"}})

		assert.Error(t, registerError)
	})
}
