package persistence_test

import (
	"testing"
	"time"

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

// watched is a symbol the system keeps up to date, registered at the given moment.
// Registration time is stated rather than taken from the clock, because the order it
// produces is the whole point of the tests below.
func watched(symbol string, market string, registeredAt time.Time) entities.TradingSymbol {
	return entities.TradingSymbol{
		Symbol: symbol, Market: market, IsWatched: true, RegisteredAt: registeredAt,
	}
}

func TestFindWatched(t *testing.T) {
	t.Run("returns only the symbols being kept up to date", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		registeredAt := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
		require.NoError(t, repository.Save(t.Context(), watched("2330", "taiwanStock", registeredAt)))
		require.NoError(t, repository.Save(t.Context(), entities.TradingSymbol{
			Symbol: "2454", Market: "taiwanStock", IsWatched: false, RegisteredAt: registeredAt,
		}))

		watchedSymbols, findError := repository.FindWatched(t.Context())

		require.NoError(t, findError)
		require.Len(t, watchedSymbols, 1)
		assert.Equal(t, "2330", watchedSymbols[0].Symbol)
		assert.Equal(t, "taiwanStock", watchedSymbols[0].Market)
	})

	t.Run("hands them back earliest registered first", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		// The names run the other way to the registrations on purpose. Symbols whose
		// alphabetical order matched their registration order would pass this whether
		// or not registration order was consulted at all.
		require.NoError(t, repository.Save(t.Context(),
			watched("2454", "taiwanStock", time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC))))
		require.NoError(t, repository.Save(t.Context(),
			watched("2330", "taiwanStock", time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC))))

		watchedSymbols, findError := repository.FindWatched(t.Context())

		require.NoError(t, findError)
		assert.Equal(t, []string{"2454", "2330"}, namesOfWatched(watchedSymbols))
	})

	t.Run("settles symbols registered at the same moment by name", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		// Everything registered before registration time was recorded ties on it. An
		// order left to the database would put a different set of symbols in a
		// market's follow places on different days.
		sameMoment := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
		require.NoError(t, repository.Save(t.Context(), watched("ETHUSDT", "crypto", sameMoment)))
		require.NoError(t, repository.Save(t.Context(), watched("BTCUSDT", "crypto", sameMoment)))

		watchedSymbols, findError := repository.FindWatched(t.Context())

		require.NoError(t, findError)
		assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, namesOfWatched(watchedSymbols))
	})

	t.Run("returns an empty list when nothing is being kept up to date", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))

		watchedSymbols, findError := repository.FindWatched(t.Context())

		require.NoError(t, findError)
		assert.Empty(t, watchedSymbols)
	})
}

func namesOfWatched(tradingSymbols []entities.TradingSymbol) []string {
	names := make([]string, 0, len(tradingSymbols))
	for _, tradingSymbol := range tradingSymbols {
		names = append(names, tradingSymbol.Symbol)
	}

	return names
}

func TestFindBySymbol(t *testing.T) {
	t.Run("returns the registered symbol with everything recorded about it", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		registeredAt := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
		require.NoError(t, repository.Save(t.Context(), watched("2330", "taiwanStock", registeredAt)))

		tradingSymbol, isRegistered, findError := repository.FindBySymbol(t.Context(), "2330")

		require.NoError(t, findError)
		assert.True(t, isRegistered)
		assert.Equal(t, "taiwanStock", tradingSymbol.Market)
		assert.True(t, tradingSymbol.IsWatched)
		assert.Equal(t, registeredAt, tradingSymbol.RegisteredAt.UTC())
	})

	t.Run("reports a symbol nobody registered without failing", func(t *testing.T) {
		// Not being registered is an answer, not a breakage: it is what "the system
		// does not know this market" looks like.
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))

		_, isRegistered, findError := repository.FindBySymbol(t.Context(), "2330")

		require.NoError(t, findError)
		assert.False(t, isRegistered)
	})
}

func TestSaveTradingSymbol(t *testing.T) {
	t.Run("replaces what was held under the same name rather than adding a second", func(t *testing.T) {
		repository := persistence.NewTradingSymbolRepository(newTestDatabase(t))
		registeredAt := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
		require.NoError(t, repository.Save(t.Context(), watched("2330", "taiwanStock", registeredAt)))

		require.NoError(t, repository.Save(t.Context(), entities.TradingSymbol{
			Symbol: "2330", Market: "taiwanStock", IsWatched: false, RegisteredAt: registeredAt,
		}))

		assert.Equal(t, []string{"2330"}, registeredNames(t, repository))
		tradingSymbol, _, findError := repository.FindBySymbol(t.Context(), "2330")
		require.NoError(t, findError)
		assert.False(t, tradingSymbol.IsWatched)
	})
}
