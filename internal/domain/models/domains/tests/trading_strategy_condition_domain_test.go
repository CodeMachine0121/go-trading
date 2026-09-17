package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// comparing is the smallest condition there is: one source equal to one signal.
func comparing(sourceLabel string, signal vo.SignalVo) dto.TradingStrategyConditionDto {
	return dto.TradingStrategyConditionDto{SourceLabel: sourceLabel, Signal: string(signal)}
}

// joining is a group of conditions under one operator.
func joining(
	operator vo.ConditionOperatorVo, conditions ...dto.TradingStrategyConditionDto,
) dto.TradingStrategyConditionDto {
	return dto.TradingStrategyConditionDto{Operator: string(operator), Conditions: conditions}
}

// nestedToDepth builds a condition nested exactly this many levels, so that a test
// about a depth limit says the depth it means instead of drawing it.
func nestedToDepth(depth int) dto.TradingStrategyConditionDto {
	condition := comparing("A", vo.SignalBuy)
	for level := 1; level < depth; level++ {
		condition = joining(vo.ConditionOperatorAnd, condition, comparing("B", vo.SignalBuy))
	}

	return condition
}

func TestTradingStrategyConditionHolds(t *testing.T) {
	testCases := []struct {
		name           string
		condition      dto.TradingStrategyConditionDto
		signalsByLabel map[string]vo.SignalVo
		expectedToHold bool
	}{
		{
			name:           "a comparison holds when that source said that signal",
			condition:      comparing("A", vo.SignalBuy),
			signalsByLabel: map[string]vo.SignalVo{"A": vo.SignalBuy},
			expectedToHold: true,
		},
		{
			name:           "a comparison does not hold when that source said something else",
			condition:      comparing("A", vo.SignalBuy),
			signalsByLabel: map[string]vo.SignalVo{"A": vo.SignalSell},
			expectedToHold: false,
		},
		{
			// Hold is one of the three values a signal has, so comparing against it
			// is an ordinary condition — not a way of saying "nothing happened".
			name:           "hold is a value a condition may compare against",
			condition:      comparing("A", vo.SignalHold),
			signalsByLabel: map[string]vo.SignalVo{"A": vo.SignalHold},
			expectedToHold: true,
		},
		{
			name:      "and holds only when every condition inside holds",
			condition: joining(vo.ConditionOperatorAnd, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			signalsByLabel: map[string]vo.SignalVo{
				"A": vo.SignalBuy, "B": vo.SignalBuy,
			},
			expectedToHold: true,
		},
		{
			name:      "and does not hold when one of them does not",
			condition: joining(vo.ConditionOperatorAnd, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			signalsByLabel: map[string]vo.SignalVo{
				"A": vo.SignalBuy, "B": vo.SignalHold,
			},
			expectedToHold: false,
		},
		{
			name:      "or holds as soon as one of them does",
			condition: joining(vo.ConditionOperatorOr, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			signalsByLabel: map[string]vo.SignalVo{
				"A": vo.SignalSell, "B": vo.SignalBuy,
			},
			expectedToHold: true,
		},
		{
			name:      "or does not hold when none of them does",
			condition: joining(vo.ConditionOperatorOr, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			signalsByLabel: map[string]vo.SignalVo{
				"A": vo.SignalSell, "B": vo.SignalHold,
			},
			expectedToHold: false,
		},
		{
			// The whole point of nesting: the bracketed half fails and the bot still
			// concludes, because the other side of the or carries it.
			name: "a nested group is read as the brackets say",
			condition: joining(vo.ConditionOperatorOr,
				joining(vo.ConditionOperatorAnd, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
				comparing("C", vo.SignalBuy)),
			signalsByLabel: map[string]vo.SignalVo{
				"A": vo.SignalHold, "B": vo.SignalHold, "C": vo.SignalBuy,
			},
			expectedToHold: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			condition, buildError := domains.NewTradingStrategyConditionDomain(
				testCase.condition, []string{"A", "B", "C"})
			require.NoError(t, buildError)

			assert.Equal(t, testCase.expectedToHold, condition.Holds(testCase.signalsByLabel))
		})
	}
}

func TestNewTradingStrategyConditionDomainRefusals(t *testing.T) {
	testCases := []struct {
		name            string
		condition       dto.TradingStrategyConditionDto
		expectedMessage string
	}{
		{
			name:            "a condition that is neither a comparison nor a group",
			condition:       dto.TradingStrategyConditionDto{},
			expectedMessage: "條件必須指名一個信號來源",
		},
		{
			name:            "a comparison naming a source that was never declared",
			condition:       comparing("D", vo.SignalBuy),
			expectedMessage: "沒有宣告的信號來源",
		},
		{
			name:            "a comparison against something that is not a signal",
			condition:       dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "maybe"},
			expectedMessage: "只能是買入、賣出或持有",
		},
		{
			name:            "an operator that is neither and nor or",
			condition:       joining("xor", comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			expectedMessage: "只能是「且」或「或」",
		},
		{
			// A group of one holds exactly when its single condition holds, so
			// allowing it would give one condition unboundedly many spellings.
			name:            "a group joining only one condition",
			condition:       joining(vo.ConditionOperatorAnd, comparing("A", vo.SignalBuy)),
			expectedMessage: "至少要 2 個子條件",
		},
		{
			name:            "a group joining nothing at all",
			condition:       joining(vo.ConditionOperatorOr),
			expectedMessage: "至少要 2 個子條件",
		},
		{
			name:            "nesting one level deeper than allowed",
			condition:       nestedToDepth(6),
			expectedMessage: "巢狀深度上限是 5 層",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewTradingStrategyConditionDomain(
				testCase.condition, []string{"A", "B", "C"})

			require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestNewTradingStrategyConditionDomainAcceptsTheDeepestAllowedNesting(t *testing.T) {
	condition, buildError := domains.NewTradingStrategyConditionDomain(
		nestedToDepth(5), []string{"A", "B"})

	require.NoError(t, buildError)
	assert.Equal(t, 5, condition.Depth())
}

func TestNewTradingStrategyConditionDomainRefusesATreeWithTooManyNodes(t *testing.T) {
	// Thirty-two comparisons under one group is thirty-three nodes: one over the
	// ceiling, and only two levels deep, so nothing but the size can refuse it.
	comparisons := make([]dto.TradingStrategyConditionDto, 0, 32)
	for range 32 {
		comparisons = append(comparisons, comparing("A", vo.SignalBuy))
	}

	_, buildError := domains.NewTradingStrategyConditionDomain(
		joining(vo.ConditionOperatorOr, comparisons...), []string{"A"})

	require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, buildError, "節點數上限是 32 個")
}

func TestTradingStrategyConditionNodeCountCountsEveryConditionIncludingItself(t *testing.T) {
	condition, buildError := domains.NewTradingStrategyConditionDomain(
		joining(vo.ConditionOperatorOr,
			joining(vo.ConditionOperatorAnd, comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
			comparing("C", vo.SignalBuy)),
		[]string{"A", "B", "C"})
	require.NoError(t, buildError)

	assert.Equal(t, 5, condition.NodeCount())
}

func TestTradingStrategyConditionToEntityKeepsTheShapeAndTheOrder(t *testing.T) {
	condition, buildError := domains.NewTradingStrategyConditionDomain(
		joining(vo.ConditionOperatorOr, comparing("A", vo.SignalBuy), comparing("B", vo.SignalSell)),
		[]string{"A", "B"})
	require.NoError(t, buildError)

	rootNode := condition.ToEntity(vo.TradingStrategyConditionSideBuy, 0)

	assert.Equal(t, string(vo.TradingStrategyConditionSideBuy), rootNode.Side)
	assert.Equal(t, string(vo.ConditionOperatorOr), rootNode.Operator)
	assert.True(t, rootNode.IsGroup())
	require.Len(t, rootNode.Children, 2)

	assert.Equal(t, "A", rootNode.Children[0].SourceLabel)
	assert.Equal(t, string(vo.SignalBuy), rootNode.Children[0].ExpectedSignal)
	assert.Equal(t, 0, rootNode.Children[0].Position)
	assert.False(t, rootNode.Children[0].IsGroup())

	assert.Equal(t, "B", rootNode.Children[1].SourceLabel)
	assert.Equal(t, string(vo.SignalSell), rootNode.Children[1].ExpectedSignal)
	assert.Equal(t, 1, rootNode.Children[1].Position)
}
