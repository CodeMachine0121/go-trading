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

	// A rewrite reaches the leverage but never the kind: the kind is settled once.
	rewrite := savedBot
	rewrite.MarketDataKind = string(vo.MarketDataKindKCandle)
	rewrite.PositionPlanLeverage = decimal.NewFromInt(3)

	rewrittenBot, rewriteError := repository.Save(t.Context(), rewrite)
	require.NoError(t, rewriteError)
	assert.Equal(t, string(vo.MarketDataKindContractKCandle), rewrittenBot.MarketDataKind)
	assert.Equal(t, "3", rewrittenBot.PositionPlanLeverage.String())
}

// A bot saved without saying what it eats is stored as the K candle — the kind of
// every bot stored before there was a choice.
func TestStrategyBotRepositoryStoresABotThatSaidNothingAsASpotBot(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)

	savedBot, saveError := repository.Save(t.Context(), aBotRow("早盤突破"))

	require.NoError(t, saveError)
	assert.Equal(t, string(vo.MarketDataKindKCandle), savedBot.ToDto().MarketDataKind)
	assert.True(t, savedBot.PositionPlanLeverage.IsZero())
}
