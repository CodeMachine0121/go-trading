package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradeTagSelectionDomain(t *testing.T) {
	breakout := entities.TradeTag{ID: 4, OwnerID: 7, Kind: "setup", Name: "突破"}
	chasing := entities.TradeTag{ID: 3, OwnerID: 7, Kind: "mistake", Name: "追價進場"}
	strangers := entities.TradeTag{ID: 9, OwnerID: 8, Kind: "setup", Name: "回踩"}
	selection := domains.NewTradeTagSelectionDomain([]entities.TradeTag{breakout, chasing, strangers}, 7)

	t.Run("the person's own tags come back in the order asked", func(t *testing.T) {
		tags, err := selection.Select([]uint{3, 4})

		require.NoError(t, err)
		assert.Equal(t, []entities.TradeTag{chasing, breakout}, tags)
	})

	t.Run("somebody else's or a missing tag is not found", func(t *testing.T) {
		_, strangerError := selection.Select([]uint{4, 9})
		_, missingError := selection.Select([]uint{5})

		require.ErrorIs(t, strangerError, domains.ErrTradeTagNotFound)
		require.ErrorIs(t, missingError, domains.ErrTradeTagNotFound)
	})

	t.Run("asking for nothing selects nothing", func(t *testing.T) {
		tags, err := selection.Select(nil)

		require.NoError(t, err)
		assert.Empty(t, tags)
	})
}

func TestTradingStrategyNamesDomain(t *testing.T) {
	known, deleted := uint(11), uint(12)
	names := domains.NewTradingStrategyNamesDomain([]entities.TradingStrategy{{ID: known, Name: "台積電波段"}}, true)

	testCases := []struct {
		name              string
		namesDomain       domains.TradingStrategyNamesDomain
		tradingStrategyID *uint
		expectedName      string
		expectedDeleted   bool
	}{
		{name: "a known strategy is named", namesDomain: names, tradingStrategyID: &known, expectedName: "台積電波段"},
		{name: "a strategy no longer there is deleted", namesDomain: names, tradingStrategyID: &deleted, expectedDeleted: true},
		{name: "a trade without a strategy is neither", namesDomain: names},
		{name: "unreadable names mark nothing deleted",
			namesDomain: domains.NewTradingStrategyNamesDomain(nil, false), tradingStrategyID: &deleted},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			name, isDeleted := testCase.namesDomain.Describe(testCase.tradingStrategyID)

			assert.Equal(t, testCase.expectedName, name)
			assert.Equal(t, testCase.expectedDeleted, isDeleted)
		})
	}
}
