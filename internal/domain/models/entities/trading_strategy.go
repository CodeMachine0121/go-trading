package entities

import (
	"cmp"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategy is one set of rules: a few strategy scripts' signals and two
// conditions that turn them into one conclusion.
//
// It holds no symbol, no trigger interval and no run state. Those three say which
// machine, watching which market, how often — none of which is the rule itself, and
// keeping them out is what lets several bots point at one set of rules instead of
// each carrying a copy that drifts from the others.
//
// It is a plain data model: fields, persistence mapping and shape conversion only,
// no business rules.
//
// The name carries a unique index spanning the owner, for the reason a strategy
// script's does: a name is what its owner recognises this by, nobody recognises a
// stranger's, and an index — not a read-then-write check — is what actually makes
// it unique.
type TradingStrategy struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is settled when this is created and never changes: it is not on the
	// list of columns a rewrite may touch, so "a trading strategy cannot change
	// hands" is something the write path cannot express rather than something it
	// remembers not to do.
	OwnerID uint   `gorm:"not null;index:idx_trading_strategies_owner;uniqueIndex:idx_trading_strategies_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_trading_strategies_owner_name"`
	// MarketDataKind is which kind of market its sources eat. Rows stored before there
	// was a choice read as the K candle, which is what they are.
	MarketDataKind string `gorm:"size:32;not null;default:kCandle"`
	// TradingMode is the contract trading mode a contract trading strategy's buys and
	// sells are read by. A K candle one has none.
	TradingMode string    `gorm:"size:32;not null;default:''"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// SignalSources and ConditionNodes belong to this trading strategy and to
	// nothing else: they are never read, created or deleted on their own, which is
	// why they have no repository of their own and travel with what owns them.
	SignalSources  []TradingStrategySignalSource  `gorm:"foreignKey:TradingStrategyID;constraint:OnDelete:CASCADE"`
	ConditionNodes []TradingStrategyConditionNode `gorm:"foreignKey:TradingStrategyID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table instead of using GORM's default.
func (tradingStrategy TradingStrategy) TableName() string {
	return "TradingStrategies"
}

// ToDto converts this record into the shape the domain hands outwards, nesting the
// two condition trees back out of the flat rows they are stored as.
//
// This is the one place the tree is reassembled. Handing out the flat rows would
// push that job onto every reader — including the screen somebody builds conditions
// on — and two reassemblers eventually disagree about the same stored rows.
func (tradingStrategy TradingStrategy) ToDto() dto.TradingStrategyDto {
	// A blank kind is a row stored before there was a choice, which is a K candle one.
	marketDataKind := tradingStrategy.MarketDataKind
	if marketDataKind == "" {
		marketDataKind = string(vo.MarketDataKindKCandle)
	}

	return dto.TradingStrategyDto{
		ID:             tradingStrategy.ID,
		OwnerID:        tradingStrategy.OwnerID,
		Name:           tradingStrategy.Name,
		MarketDataKind: marketDataKind,
		TradingMode:    tradingStrategy.TradingMode,
		SignalSources:  tradingStrategy.signalSourceDtos(),
		BuyCondition:   tradingStrategy.conditionDto(vo.TradingStrategyConditionSideBuy),
		SellCondition:  tradingStrategy.conditionDto(vo.TradingStrategyConditionSideSell),
		CreatedAt:      tradingStrategy.CreatedAt.UTC(),
		UpdatedAt:      tradingStrategy.UpdatedAt.UTC(),
	}
}

// signalSourceDtos hands out this trading strategy's sources, always as a list
// rather than sometimes nothing.
func (tradingStrategy TradingStrategy) signalSourceDtos() []dto.TradingStrategySignalSourceDto {
	signalSourceDtos := make([]dto.TradingStrategySignalSourceDto, 0, len(tradingStrategy.SignalSources))
	for _, signalSource := range tradingStrategy.SignalSources {
		signalSourceDtos = append(signalSourceDtos, signalSource.ToDto())
	}

	return signalSourceDtos
}

// conditionDto rebuilds one of the two trees from the flat rows.
//
// It indexes the children by parent once and then descends, rather than scanning the
// whole slice at every node: a tree of 32 nodes scanned per node is a thousand
// comparisons for something read on every list.
func (tradingStrategy TradingStrategy) conditionDto(
	side vo.TradingStrategyConditionSideVo,
) dto.TradingStrategyConditionDto {
	childrenByParent := map[uint][]TradingStrategyConditionNode{}
	root := TradingStrategyConditionNode{}
	hasRoot := false

	for _, node := range tradingStrategy.ConditionNodes {
		if vo.TradingStrategyConditionSideVo(node.Side) != side {
			continue
		}

		if node.ParentID == nil {
			root = node
			hasRoot = true

			continue
		}

		childrenByParent[*node.ParentID] = append(childrenByParent[*node.ParentID], node)
	}

	if !hasRoot {
		return dto.TradingStrategyConditionDto{}
	}

	return root.toDto(childrenByParent)
}

// toDto turns this node and everything under it into the nested shape. It is on the
// node because it reads the node's own fields; the map of children is the only thing
// it cannot get from itself.
func (tradingStrategyConditionNode TradingStrategyConditionNode) toDto(
	childrenByParent map[uint][]TradingStrategyConditionNode,
) dto.TradingStrategyConditionDto {
	if !tradingStrategyConditionNode.IsGroup() {
		return dto.TradingStrategyConditionDto{
			SourceLabel: tradingStrategyConditionNode.SourceLabel,
			Signal:      tradingStrategyConditionNode.ExpectedSignal,
		}
	}

	// Siblings are ordered here rather than trusted to arrive ordered. The order
	// changes no answer — and and or do not care — but it is what somebody sees
	// when they open the condition again, and a tree that rearranges itself
	// between two reads looks like it was edited by somebody else.
	children := childrenByParent[tradingStrategyConditionNode.ID]
	slices.SortFunc(children, func(left TradingStrategyConditionNode, right TradingStrategyConditionNode) int {
		return cmp.Compare(left.Position, right.Position)
	})

	conditionDtos := make([]dto.TradingStrategyConditionDto, 0, len(children))
	for _, child := range children {
		conditionDtos = append(conditionDtos, child.toDto(childrenByParent))
	}

	return dto.TradingStrategyConditionDto{
		Operator:   tradingStrategyConditionNode.Operator,
		Conditions: conditionDtos,
	}
}
