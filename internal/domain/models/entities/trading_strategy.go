package entities

import (
	"cmp"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategy is a set of rules (signal sources and buy/sell conditions); it holds no symbol or schedule so several bots can share it.
type TradingStrategy struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is never on the rewrite column list, so a trading strategy cannot change hands.
	OwnerID uint   `gorm:"not null;index:idx_trading_strategies_owner;uniqueIndex:idx_trading_strategies_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_trading_strategies_owner_name"`
	// MarketDataKind blank means a row stored before the choice existed, i.e. K candle.
	MarketDataKind string `gorm:"size:32;not null;default:kCandle"`
	// TradingMode applies only to contract strategies; it uses a new column so stale spot-era values in the retired trading_mode column are never read.
	TradingMode string    `gorm:"column:contract_trading_mode;size:32;not null;default:''"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`

	Owner          User                           `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	SignalSources  []TradingStrategySignalSource  `gorm:"foreignKey:TradingStrategyID;constraint:OnDelete:CASCADE"`
	ConditionNodes []TradingStrategyConditionNode `gorm:"foreignKey:TradingStrategyID;constraint:OnDelete:CASCADE"`
}

func (tradingStrategy TradingStrategy) TableName() string {
	return "TradingStrategies"
}

// ToDto nests the two condition trees back out of their flat rows; this is the only place the tree is reassembled.
func (tradingStrategy TradingStrategy) ToDto() dto.TradingStrategyDto {
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

// signalSourceDtos always returns a non-nil list.
func (tradingStrategy TradingStrategy) signalSourceDtos() []dto.TradingStrategySignalSourceDto {
	signalSourceDtos := make([]dto.TradingStrategySignalSourceDto, 0, len(tradingStrategy.SignalSources))
	for _, signalSource := range tradingStrategy.SignalSources {
		signalSourceDtos = append(signalSourceDtos, signalSource.ToDto())
	}

	return signalSourceDtos
}

// conditionDto rebuilds one tree by indexing children by parent once, rather than rescanning all rows at every node.
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

func (tradingStrategyConditionNode TradingStrategyConditionNode) toDto(
	childrenByParent map[uint][]TradingStrategyConditionNode,
) dto.TradingStrategyConditionDto {
	if !tradingStrategyConditionNode.IsGroup() {
		return dto.TradingStrategyConditionDto{
			SourceLabel: tradingStrategyConditionNode.SourceLabel,
			Signal:      tradingStrategyConditionNode.ExpectedSignal,
		}
	}

	// Sort siblings so the tree reads back in the order it was written, whatever order the rows arrived in.
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
