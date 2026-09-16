package entities_test

import (
	"testing"
	"time"

	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identifierOf is a pointer to a stored identifier, which is how a node names its
// parent and how a root says it has none.
func identifierOf(id uint) *uint {
	return &id
}

// aStoredTree is the flat rows one bot's two condition trees are kept as:
//
//	buy:  ( A=buy or B=buy )
//	sell: C=sell
//
// The rows are deliberately out of order, because a table hands them back in
// whatever order it likes and the nesting must not depend on that.
func aStoredTree() []StrategyBotConditionNode {
	return []StrategyBotConditionNode{
		{ID: 12, StrategyBotID: 3, Side: string(vo.StrategyBotConditionSideBuy),
			ParentID: identifierOf(10), Position: 1, SourceLabel: "B", ExpectedSignal: "buy"},
		{ID: 20, StrategyBotID: 3, Side: string(vo.StrategyBotConditionSideSell),
			Position: 0, SourceLabel: "C", ExpectedSignal: "sell"},
		{ID: 10, StrategyBotID: 3, Side: string(vo.StrategyBotConditionSideBuy),
			Position: 0, Operator: string(vo.ConditionOperatorOr)},
		{ID: 11, StrategyBotID: 3, Side: string(vo.StrategyBotConditionSideBuy),
			ParentID: identifierOf(10), Position: 0, SourceLabel: "A", ExpectedSignal: "buy"},
	}
}

func TestStrategyBotToDtoNestsTheTreesBackOutOfTheFlatRows(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破", Symbol: "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		ConditionNodes:         aStoredTree(),
	}

	botDto := strategyBot.ToDto()

	assert.Equal(t, string(vo.ConditionOperatorOr), botDto.BuyCondition.Operator)
	require.Len(t, botDto.BuyCondition.Conditions, 2)
	// Siblings come back in the order they were written, whatever order the table
	// handed them over in — a condition that rearranges itself between two reads
	// looks like it was edited by somebody else.
	assert.Equal(t, "A", botDto.BuyCondition.Conditions[0].SourceLabel)
	assert.Equal(t, "B", botDto.BuyCondition.Conditions[1].SourceLabel)
	assert.Empty(t, botDto.BuyCondition.SourceLabel)

	assert.Equal(t, "C", botDto.SellCondition.SourceLabel)
	assert.Equal(t, "sell", botDto.SellCondition.Signal)
	assert.Empty(t, botDto.SellCondition.Operator)
}

func TestStrategyBotToDtoCarriesWhatTheListIsReadFor(t *testing.T) {
	strategyBot := StrategyBot{
		ID: 3, OwnerID: 7, Name: "早盤突破", Symbol: "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		LastSentSignal:         string(vo.SignalBuy),
		HaltReason:             string(vo.StrategyBotHaltScriptFailed),
		Conflicting:            true,
		CreatedAt:              time.Date(2026, 9, 16, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		UpdatedAt:              time.Date(2026, 9, 16, 9, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		ConditionNodes:         aStoredTree(),
	}

	botDto := strategyBot.ToDto()

	// Both times are handed out in universal time whatever zone they were read back
	// in, so two people in two places read the same moment.
	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), botDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC), botDto.UpdatedAt)

	// A list of bots is read to answer one question — which of these needs looking
	// at — so every part of that answer travels with the bot.
	assert.Equal(t, uint(7), botDto.OwnerID)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
	assert.Equal(t, string(vo.SignalBuy), botDto.LastSentSignal)
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), botDto.HaltReason)
	assert.True(t, botDto.Conflicting)
}

func TestStrategyBotToDtoHandsOutAnEmptyListRatherThanNothing(t *testing.T) {
	botDto := StrategyBot{ID: 3}.ToDto()

	assert.NotNil(t, botDto.SignalSources)
	assert.Empty(t, botDto.SignalSources)
}

func TestStrategyBotSignalSourceToDtoCarriesNoScriptAndNoStaleName(t *testing.T) {
	signalSource := StrategyBotSignalSource{
		Label: "均線黃金交叉", StrategyID: 9,
		AggregationInterval: string(vo.AggregationIntervalOneHour),
		ParameterValues: []StrategyBotSignalSourceParameterValue{
			{Name: "回看根數", Value: 20},
		},
	}

	sourceDto := signalSource.ToDto()

	assert.Equal(t, "均線黃金交叉", sourceDto.Label)
	assert.Equal(t, uint(9), sourceDto.StrategyID)
	assert.Equal(t, string(vo.AggregationIntervalOneHour), sourceDto.AggregationInterval)
	require.Len(t, sourceDto.ParameterValues, 1)
	assert.Equal(t, "回看根數", sourceDto.ParameterValues[0].Name)
	assert.Equal(t, 20.0, sourceDto.ParameterValues[0].Value)
}

func TestStrategyBotConditionNodeIsGroupReadsTheOneFieldThatDecides(t *testing.T) {
	assert.True(t, StrategyBotConditionNode{Operator: string(vo.ConditionOperatorAnd)}.IsGroup())
	assert.False(t, StrategyBotConditionNode{SourceLabel: "A", ExpectedSignal: "buy"}.IsGroup())
}
