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

// aStoredTree is the flat rows one trading strategy's two condition trees are kept as:
//
//	buy:  ( A=buy or B=buy )
//	sell: C=sell
//
// The rows are deliberately out of order, because a table hands them back in
// whatever order it likes and the nesting must not depend on that.
func aStoredTree() []TradingStrategyConditionNode {
	return []TradingStrategyConditionNode{
		{ID: 12, TradingStrategyID: 3, Side: string(vo.TradingStrategyConditionSideBuy),
			ParentID: identifierOf(10), Position: 1, SourceLabel: "B", ExpectedSignal: "buy"},
		{ID: 20, TradingStrategyID: 3, Side: string(vo.TradingStrategyConditionSideSell),
			Position: 0, SourceLabel: "C", ExpectedSignal: "sell"},
		{ID: 10, TradingStrategyID: 3, Side: string(vo.TradingStrategyConditionSideBuy),
			Position: 0, Operator: string(vo.ConditionOperatorOr)},
		{ID: 11, TradingStrategyID: 3, Side: string(vo.TradingStrategyConditionSideBuy),
			ParentID: identifierOf(10), Position: 0, SourceLabel: "A", ExpectedSignal: "buy"},
	}
}

func TestTradingStrategyToDtoNestsTheTreesBackOutOfTheFlatRows(t *testing.T) {
	tradingStrategy := TradingStrategy{
		ID: 3, OwnerID: 7, Name: "早盤突破",
		ConditionNodes: aStoredTree(),
	}

	tradingStrategyDto := tradingStrategy.ToDto()

	assert.Equal(t, string(vo.ConditionOperatorOr), tradingStrategyDto.BuyCondition.Operator)
	require.Len(t, tradingStrategyDto.BuyCondition.Conditions, 2)
	// Siblings come back in the order they were written, whatever order the table
	// handed them over in — a condition that rearranges itself between two reads
	// looks like it was edited by somebody else.
	assert.Equal(t, "A", tradingStrategyDto.BuyCondition.Conditions[0].SourceLabel)
	assert.Equal(t, "B", tradingStrategyDto.BuyCondition.Conditions[1].SourceLabel)
	assert.Empty(t, tradingStrategyDto.BuyCondition.SourceLabel)

	assert.Equal(t, "C", tradingStrategyDto.SellCondition.SourceLabel)
	assert.Equal(t, "sell", tradingStrategyDto.SellCondition.Signal)
	assert.Empty(t, tradingStrategyDto.SellCondition.Operator)
}

func TestTradingStrategyToDtoHandsOutTimesInUniversalTime(t *testing.T) {
	tradingStrategy := TradingStrategy{
		ID: 3, OwnerID: 7, Name: "早盤突破",
		CreatedAt:      time.Date(2026, 9, 16, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		UpdatedAt:      time.Date(2026, 9, 16, 9, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		ConditionNodes: aStoredTree(),
	}

	tradingStrategyDto := tradingStrategy.ToDto()

	// Both times are handed out in universal time whatever zone they were read back
	// in, so two people in two places read the same moment.
	assert.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), tradingStrategyDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 16, 1, 0, 0, 0, time.UTC), tradingStrategyDto.UpdatedAt)
	assert.Equal(t, uint(7), tradingStrategyDto.OwnerID)
}

func TestTradingStrategyToDtoHandsOutAnEmptyListRatherThanNothing(t *testing.T) {
	tradingStrategyDto := TradingStrategy{ID: 3}.ToDto()

	assert.NotNil(t, tradingStrategyDto.SignalSources)
	assert.Empty(t, tradingStrategyDto.SignalSources)
}

func TestTradingStrategySignalSourceToDtoCarriesNoScriptAndNoStaleName(t *testing.T) {
	signalSource := TradingStrategySignalSource{
		Label: "均線黃金交叉", StrategyScriptID: 9,
		AggregationInterval: string(vo.AggregationIntervalOneHour),
		ParameterValues: []TradingStrategySignalSourceParameterValue{
			{Name: "回看根數", Value: 20},
		},
	}

	sourceDto := signalSource.ToDto()

	assert.Equal(t, "均線黃金交叉", sourceDto.Label)
	assert.Equal(t, uint(9), sourceDto.StrategyScriptID)
	assert.Equal(t, string(vo.AggregationIntervalOneHour), sourceDto.AggregationInterval)
	require.Len(t, sourceDto.ParameterValues, 1)
	assert.Equal(t, "回看根數", sourceDto.ParameterValues[0].Name)
	assert.Equal(t, 20.0, sourceDto.ParameterValues[0].Value)
}

func TestTradingStrategyConditionNodeIsGroupReadsTheOneFieldThatDecides(t *testing.T) {
	assert.True(t, TradingStrategyConditionNode{Operator: string(vo.ConditionOperatorAnd)}.IsGroup())
	assert.False(t, TradingStrategyConditionNode{SourceLabel: "A", ExpectedSignal: "buy"}.IsGroup())
}
