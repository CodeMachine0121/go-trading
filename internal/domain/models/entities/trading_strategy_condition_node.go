package entities

// TradingStrategyConditionNode is one node of a condition tree, stored flat and nested again only in TradingStrategy.ToDto; an empty Operator means a comparison, not a group.
type TradingStrategyConditionNode struct {
	ID                uint `gorm:"primaryKey"`
	TradingStrategyID uint `gorm:"not null;index:idx_trading_strategy_condition_nodes_strategy"`
	// Side says which of the two trees this node belongs to.
	Side string `gorm:"size:8;not null"`
	// ParentID is nil on a root; it self-cascades so deleting the roots removes the whole tree.
	ParentID *uint `gorm:"index:idx_trading_strategy_condition_nodes_parent"`
	// Position orders siblings so a rewritten tree reads back in the order it was written.
	Position int    `gorm:"not null"`
	Operator string `gorm:"size:8;not null;default:''"`
	// SourceLabel and ExpectedSignal are empty on a group.
	SourceLabel    string `gorm:"size:32;not null;default:''"`
	ExpectedSignal string `gorm:"size:16;not null;default:''"`

	Parent   *TradingStrategyConditionNode  `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
	Children []TradingStrategyConditionNode `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
}

func (tradingStrategyConditionNode TradingStrategyConditionNode) TableName() string {
	return "TradingStrategyConditionNodes"
}

func (tradingStrategyConditionNode TradingStrategyConditionNode) IsGroup() bool {
	return tradingStrategyConditionNode.Operator != ""
}
