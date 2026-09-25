package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyBotRepositoryKeepsAContractBotsKindAndLeverage(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	contractBot := aBotRow("合約突破")
	contractBot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	contractBot.PositionPlanLeverage = decimal.NewFromInt(5)

	savedBot, saveError := repository.Save(t.Context(), contractBot)
	require.NoError(t, saveError)
	assert.Equal(t, string(vo.MarketDataKindContractKCandle), savedBot.MarketDataKind)
	assert.Equal(t, "5", savedBot.PositionPlanLeverage.String())

	// A rewrite may change the leverage but never the market data kind.
	rewrite := savedBot
	rewrite.MarketDataKind = string(vo.MarketDataKindKCandle)
	rewrite.PositionPlanLeverage = decimal.NewFromInt(3)

	rewrittenBot, rewriteError := repository.Save(t.Context(), rewrite)
	require.NoError(t, rewriteError)
	assert.Equal(t, string(vo.MarketDataKindContractKCandle), rewrittenBot.MarketDataKind)
	assert.Equal(t, "3", rewrittenBot.PositionPlanLeverage.String())
}

// A bot saved without a market data kind defaults to K candles, as bots stored before the choice existed.
func TestStrategyBotRepositoryStoresABotThatSaidNothingAsASpotBot(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))

	require.NoError(t, saveError)
	assert.Equal(t, string(vo.MarketDataKindKCandle), savedBot.ToDto().MarketDataKind)
	assert.True(t, savedBot.PositionPlanLeverage.IsZero())
}
