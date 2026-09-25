package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotConditionMaxDepth counts the outermost condition as level one; deeper logic
// belongs in an indicator script.
const strategyBotConditionMaxDepth = 5

// strategyBotConditionMaxNodeCount bounds evaluation cost, since trees are evaluated
// unattended every few minutes.
const strategyBotConditionMaxNodeCount = 32

// strategyBotConditionGroupMinimumSize forbids single-child groups, which would allow
// endless equivalent spellings of one tree.
const strategyBotConditionGroupMinimumSize = 2

// TradingStrategyConditionDomain is a comparison or an and/or group; evaluation is pure so
// replays can feed it one round of signals at a time.
type TradingStrategyConditionDomain struct {
	// operator is empty on a comparison.
	operator       vo.ConditionOperatorVo
	children       []TradingStrategyConditionDomain
	sourceLabel    string
	expectedSignal vo.SignalVo
}

// NewTradingStrategyConditionDomain validates the whole tree recursively and refuses it if
// any part fails.
func NewTradingStrategyConditionDomain(
	conditionDto dto.TradingStrategyConditionDto, declaredLabels []string,
) (TradingStrategyConditionDomain, error) {
	operator := vo.ConditionOperatorVo(strings.TrimSpace(conditionDto.Operator))

	if operator == "" {
		sourceLabel := strings.TrimSpace(conditionDto.SourceLabel)
		if sourceLabel == "" {
			return TradingStrategyConditionDomain{}, fmt.Errorf(
				"%w: 條件必須指名一個信號來源，或是一個帶著運算子的條件群組",
				ErrTradingStrategyValidation)
		}

		// An undeclared source label would silently never hold.
		if !slices.Contains(declaredLabels, sourceLabel) {
			return TradingStrategyConditionDomain{}, fmt.Errorf(
				"%w: 條件指到了一個沒有宣告的信號來源 %q", ErrTradingStrategyValidation, sourceLabel)
		}

		expectedSignal := vo.SignalVo(strings.TrimSpace(conditionDto.Signal))
		if expectedSignal != vo.SignalBuy &&
			expectedSignal != vo.SignalSell &&
			expectedSignal != vo.SignalHold {
			return TradingStrategyConditionDomain{}, fmt.Errorf(
				"%w: 條件比對的信號只能是買入、賣出或持有，收到的是 %q",
				ErrTradingStrategyValidation, conditionDto.Signal)
		}

		return TradingStrategyConditionDomain{
			sourceLabel:    sourceLabel,
			expectedSignal: expectedSignal,
		}, nil
	}

	if operator != vo.ConditionOperatorAnd && operator != vo.ConditionOperatorOr {
		return TradingStrategyConditionDomain{}, fmt.Errorf(
			"%w: 條件群組的運算子只能是「且」或「或」，收到的是 %q",
			ErrTradingStrategyValidation, conditionDto.Operator)
	}

	if len(conditionDto.Conditions) < strategyBotConditionGroupMinimumSize {
		return TradingStrategyConditionDomain{}, fmt.Errorf(
			"%w: 一個條件群組至少要 %d 個子條件",
			ErrTradingStrategyValidation, strategyBotConditionGroupMinimumSize)
	}

	children := make([]TradingStrategyConditionDomain, 0, len(conditionDto.Conditions))
	for _, childDto := range conditionDto.Conditions {
		child, childError := NewTradingStrategyConditionDomain(childDto, declaredLabels)
		if childError != nil {
			return TradingStrategyConditionDomain{}, childError
		}

		children = append(children, child)
	}

	condition := TradingStrategyConditionDomain{operator: operator, children: children}

	if condition.Depth() > strategyBotConditionMaxDepth {
		return TradingStrategyConditionDomain{}, fmt.Errorf(
			"%w: 條件的巢狀深度上限是 %d 層", ErrTradingStrategyValidation, strategyBotConditionMaxDepth)
	}

	if condition.NodeCount() > strategyBotConditionMaxNodeCount {
		return TradingStrategyConditionDomain{}, fmt.Errorf(
			"%w: 一棵條件樹的節點數上限是 %d 個",
			ErrTradingStrategyValidation, strategyBotConditionMaxNodeCount)
	}

	return condition, nil
}

// Holds treats a missing source signal as false, which is unreachable in practice because
// rounds abandon before evaluating without every signal.
func (tradingStrategyConditionDomain TradingStrategyConditionDomain) Holds(
	signalsByLabel map[string]vo.SignalVo,
) bool {
	if tradingStrategyConditionDomain.operator == "" {
		return signalsByLabel[tradingStrategyConditionDomain.sourceLabel] ==
			tradingStrategyConditionDomain.expectedSignal
	}

	if tradingStrategyConditionDomain.operator == vo.ConditionOperatorAnd {
		for _, child := range tradingStrategyConditionDomain.children {
			if !child.Holds(signalsByLabel) {
				return false
			}
		}

		return true
	}

	for _, child := range tradingStrategyConditionDomain.children {
		if child.Holds(signalsByLabel) {
			return true
		}
	}

	return false
}

func (tradingStrategyConditionDomain TradingStrategyConditionDomain) Depth() int {
	deepestChild := 0
	for _, child := range tradingStrategyConditionDomain.children {
		childDepth := child.Depth()
		if childDepth > deepestChild {
			deepestChild = childDepth
		}
	}

	return deepestChild + 1
}

func (tradingStrategyConditionDomain TradingStrategyConditionDomain) NodeCount() int {
	nodeCount := 1
	for _, child := range tradingStrategyConditionDomain.children {
		nodeCount += child.NodeCount()
	}

	return nodeCount
}

// ToEntity nests rows through their children so the writer can assign parent IDs; IDs are
// left to the store.
func (tradingStrategyConditionDomain TradingStrategyConditionDomain) ToEntity(
	side vo.TradingStrategyConditionSideVo, position int,
) entities.TradingStrategyConditionNode {
	node := entities.TradingStrategyConditionNode{
		Side:           string(side),
		Position:       position,
		Operator:       string(tradingStrategyConditionDomain.operator),
		SourceLabel:    tradingStrategyConditionDomain.sourceLabel,
		ExpectedSignal: string(tradingStrategyConditionDomain.expectedSignal),
	}

	children := make([]entities.TradingStrategyConditionNode, 0, len(tradingStrategyConditionDomain.children))
	for childPosition, child := range tradingStrategyConditionDomain.children {
		children = append(children, child.ToEntity(side, childPosition))
	}

	node.Children = children

	return node
}
