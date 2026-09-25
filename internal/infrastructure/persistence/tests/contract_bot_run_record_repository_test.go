package persistence_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aContractSuggestion is short at 5x leverage, 5000 notional on 1000 margin, stop at 102.
func aContractSuggestion() dto.PositionPlanDto {
	return dto.PositionPlanDto{
		Stake: decimal.NewFromInt(1000), Affordable: true,
		StopLossPrice: decimal.NewFromInt(102), HasStopLoss: true,
		Direction: "short", Leverage: decimal.NewFromInt(5), Notional: decimal.NewFromInt(5000),
		ForContract: true,
	}
}

func TestStrategyBotRunRecordRepositoryRemembersWhichWayAndHowFarAContractRoundLeaned(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "sell",
		HasPositionPlan: true, PositionPlan: aContractSuggestion(),
	}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)

	runRecordDto := runRecords[0].ToDto()
	assert.Equal(t, "short", runRecordDto.SuggestedDirection)
	require.NotNil(t, runRecordDto.SuggestedLeverage)
	assert.Equal(t, "5", runRecordDto.SuggestedLeverage.String())
	require.NotNil(t, runRecordDto.SuggestedNotional)
	assert.Equal(t, "5000", runRecordDto.SuggestedNotional.String())
	assert.Equal(t, "1000", runRecordDto.SuggestedStake.String())
}

// A spot suggestion stores no direction, leverage or notional, and none of them appear on the wire.
func TestStrategyBotRunRecordRepositoryKeepsASpotRoundAsItWas(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	spotSuggestion := dto.PositionPlanDto{
		Stake: decimal.NewFromInt(5000), Affordable: true,
		Direction: "long", Leverage: decimal.NewFromInt(1), Notional: decimal.NewFromInt(5000),
	}
	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "buy",
		HasPositionPlan: true, PositionPlan: spotSuggestion,
	}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)

	wire, marshalError := json.Marshal(runRecords[0].ToDto())
	require.NoError(t, marshalError)
	assert.Contains(t, string(wire), `"suggestedStake":"5000"`)
	assert.NotContains(t, string(wire), "suggestedDirection")
	assert.NotContains(t, string(wire), "suggestedLeverage")
	assert.NotContains(t, string(wire), "suggestedNotional")
}

func TestStrategyBotRunRecordRepositoryRemembersNothingTheVenueWouldHaveRefused(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotToRecordAgainst(t, database)
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	refused := aContractSuggestion()
	refused.TakeProfitPrice = decimal.NewFromInt(96)
	refused.HasTakeProfit = true
	refused.HasVenueRefusal = true
	refused.VenueRefusal = dto.ContractOrderRefusalDto{Reason: "belowMinimumNotional"}
	require.NoError(t, repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
		StrategyBotID: botID, RanAt: runRecordRanAt, Result: "buy",
		HasPositionPlan: true, PositionPlan: refused,
	}))

	runRecords, findError := repository.FindLatestByBot(t.Context(), botID)
	require.NoError(t, findError)
	require.Len(t, runRecords, 1)

	runRecordDto := runRecords[0].ToDto()
	assert.Nil(t, runRecordDto.SuggestedStake)
	assert.Nil(t, runRecordDto.SuggestedStopLossPrice)
	assert.Nil(t, runRecordDto.SuggestedTakeProfitPrice)
	assert.Empty(t, runRecordDto.SuggestedDirection)
	assert.Nil(t, runRecordDto.SuggestedLeverage)
	assert.Nil(t, runRecordDto.SuggestedNotional)
}
