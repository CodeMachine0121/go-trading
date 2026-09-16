package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotConditionMaxDepth is how deeply conditions may nest, counting the
// outermost one as the first level. Five is where a condition somebody wrote stops
// being a condition they can still read; anything that genuinely needs more belongs
// inside an indicator script, which is an actual programming language.
const strategyBotConditionMaxDepth = 5

// strategyBotConditionMaxNodeCount is how many conditions one tree may hold in
// total. A tree without a ceiling is a computation without a ceiling, and this one
// runs itself every few minutes with nobody watching.
const strategyBotConditionMaxNodeCount = 32

// strategyBotConditionGroupMinimumSize is how many conditions a group must join. A
// group of one holds exactly when the condition inside it holds, so allowing it
// would give the same tree unboundedly many spellings — and a person opening theirs
// again would find brackets they never typed.
const strategyBotConditionGroupMinimumSize = 2

// StrategyBotConditionDomain is one of a bot's two conditions: either a comparison
// against one signal source, or a group of conditions joined by and or or.
//
// An instance only exists when the whole tree passed every rule, so there is no
// half-valid condition anywhere downstream.
//
// Evaluating it touches nothing but the signals it is handed — no clock, no
// database, no network. That is what makes replaying a bot over history a matter of
// feeding it one round's signals at a time, with not a line of this changed.
type StrategyBotConditionDomain struct {
	// operator is empty on a comparison. It is the one field that says which kind
	// of condition this is.
	operator       vo.ConditionOperatorVo
	children       []StrategyBotConditionDomain
	sourceLabel    string
	expectedSignal vo.SignalVo
}

// NewStrategyBotConditionDomain validates one condition and everything under it
// against every rule that applies, and refuses the whole tree if any part fails.
//
// It recurses through itself rather than through a private helper, so the rules are
// written once and hold at every level. Checking depth and size inside a subtree as
// well as at the top costs nothing and is never wrong: a subtree is never deeper or
// larger than the tree containing it.
func NewStrategyBotConditionDomain(
	conditionDto dto.StrategyBotConditionDto, declaredLabels []string,
) (StrategyBotConditionDomain, error) {
	operator := vo.ConditionOperatorVo(strings.TrimSpace(conditionDto.Operator))

	// A condition with no operator is the smallest kind there is: one signal source
	// being equal to one signal. It is settled here and returns, so that none of
	// the group rules below have to keep saying "not for this kind".
	if operator == "" {
		sourceLabel := strings.TrimSpace(conditionDto.SourceLabel)
		if sourceLabel == "" {
			return StrategyBotConditionDomain{}, fmt.Errorf(
				"%w: 條件必須指名一個信號來源，或是一個帶著運算子的條件群組",
				ErrStrategyBotValidation)
		}

		// A condition may only name a source this bot declared. Left unchecked, it
		// would silently never hold, and its owner would spend the week wondering
		// why a bot that looks right says nothing.
		if !slices.Contains(declaredLabels, sourceLabel) {
			return StrategyBotConditionDomain{}, fmt.Errorf(
				"%w: 條件指到了一個沒有宣告的信號來源 %q", ErrStrategyBotValidation, sourceLabel)
		}

		expectedSignal := vo.SignalVo(strings.TrimSpace(conditionDto.Signal))
		if expectedSignal != vo.SignalBuy &&
			expectedSignal != vo.SignalSell &&
			expectedSignal != vo.SignalHold {
			return StrategyBotConditionDomain{}, fmt.Errorf(
				"%w: 條件比對的信號只能是買入、賣出或持有，收到的是 %q",
				ErrStrategyBotValidation, conditionDto.Signal)
		}

		return StrategyBotConditionDomain{
			sourceLabel:    sourceLabel,
			expectedSignal: expectedSignal,
		}, nil
	}

	if operator != vo.ConditionOperatorAnd && operator != vo.ConditionOperatorOr {
		return StrategyBotConditionDomain{}, fmt.Errorf(
			"%w: 條件群組的運算子只能是「且」或「或」，收到的是 %q",
			ErrStrategyBotValidation, conditionDto.Operator)
	}

	if len(conditionDto.Conditions) < strategyBotConditionGroupMinimumSize {
		return StrategyBotConditionDomain{}, fmt.Errorf(
			"%w: 一個條件群組至少要 %d 個子條件",
			ErrStrategyBotValidation, strategyBotConditionGroupMinimumSize)
	}

	children := make([]StrategyBotConditionDomain, 0, len(conditionDto.Conditions))
	for _, childDto := range conditionDto.Conditions {
		child, childError := NewStrategyBotConditionDomain(childDto, declaredLabels)
		if childError != nil {
			return StrategyBotConditionDomain{}, childError
		}

		children = append(children, child)
	}

	condition := StrategyBotConditionDomain{operator: operator, children: children}

	if condition.Depth() > strategyBotConditionMaxDepth {
		return StrategyBotConditionDomain{}, fmt.Errorf(
			"%w: 條件的巢狀深度上限是 %d 層", ErrStrategyBotValidation, strategyBotConditionMaxDepth)
	}

	if condition.NodeCount() > strategyBotConditionMaxNodeCount {
		return StrategyBotConditionDomain{}, fmt.Errorf(
			"%w: 一棵條件樹的節點數上限是 %d 個",
			ErrStrategyBotValidation, strategyBotConditionMaxNodeCount)
	}

	return condition, nil
}

// Holds says whether this condition is true given what each signal source said this
// round.
//
// A comparison against a source that said nothing is false rather than an error: by
// the time a round evaluates, every declared source has already produced a signal or
// the round has already been abandoned, so there is no reachable way to arrive here
// missing one.
func (strategyBotConditionDomain StrategyBotConditionDomain) Holds(
	signalsByLabel map[string]vo.SignalVo,
) bool {
	if strategyBotConditionDomain.operator == "" {
		return signalsByLabel[strategyBotConditionDomain.sourceLabel] ==
			strategyBotConditionDomain.expectedSignal
	}

	if strategyBotConditionDomain.operator == vo.ConditionOperatorAnd {
		for _, child := range strategyBotConditionDomain.children {
			if !child.Holds(signalsByLabel) {
				return false
			}
		}

		return true
	}

	for _, child := range strategyBotConditionDomain.children {
		if child.Holds(signalsByLabel) {
			return true
		}
	}

	return false
}

// Depth is how many levels this condition nests, counting itself as one.
func (strategyBotConditionDomain StrategyBotConditionDomain) Depth() int {
	deepestChild := 0
	for _, child := range strategyBotConditionDomain.children {
		childDepth := child.Depth()
		if childDepth > deepestChild {
			deepestChild = childDepth
		}
	}

	return deepestChild + 1
}

// NodeCount is how many conditions this one holds in total, counting itself.
func (strategyBotConditionDomain StrategyBotConditionDomain) NodeCount() int {
	nodeCount := 1
	for _, child := range strategyBotConditionDomain.children {
		nodeCount += child.NodeCount()
	}

	return nodeCount
}

// ToEntity flattens this condition into the rows it is stored as, nested through
// each node's children so that whoever writes them can walk down assigning parents.
//
// The identifiers are left unset: they belong to the store, and a model that guessed
// at them would be wrong the first time two bots were saved at once.
func (strategyBotConditionDomain StrategyBotConditionDomain) ToEntity(
	side vo.StrategyBotConditionSideVo, position int,
) entities.StrategyBotConditionNode {
	node := entities.StrategyBotConditionNode{
		Side:           string(side),
		Position:       position,
		Operator:       string(strategyBotConditionDomain.operator),
		SourceLabel:    strategyBotConditionDomain.sourceLabel,
		ExpectedSignal: string(strategyBotConditionDomain.expectedSignal),
	}

	children := make([]entities.StrategyBotConditionNode, 0, len(strategyBotConditionDomain.children))
	for childPosition, child := range strategyBotConditionDomain.children {
		children = append(children, child.ToEntity(side, childPosition))
	}

	node.Children = children

	return node
}
