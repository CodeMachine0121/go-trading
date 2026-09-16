package entities

// StrategyBotConditionNode is one node of one of a bot's two condition trees.
//
// The tree is stored flat — a parent reference and a position among siblings —
// because that is the shape a table holds. It is nested again on the way out, once,
// in StrategyBot.ToDto; nothing else reassembles it.
//
// A node is either a group or a comparison, and Operator being empty is what says
// which. Two tables, one per kind, would make every read a join of two shapes that
// are never useful apart.
type StrategyBotConditionNode struct {
	ID            uint `gorm:"primaryKey"`
	StrategyBotID uint `gorm:"not null;index:idx_strategy_bot_condition_nodes_bot"`
	// Side says which of the two trees this node belongs to. Both trees live in one
	// table because they are the same shape and are always read together; a column
	// is cheaper than a second table that would need every query written twice.
	Side string `gorm:"size:8;not null"`
	// ParentID is nil on the root of a tree. It cascades from itself so that
	// deleting a subtree takes its children with it, which is what makes rewriting
	// a condition a delete of the roots rather than a walk.
	ParentID *uint `gorm:"index:idx_strategy_bot_condition_nodes_parent"`
	// Position orders siblings. It has no meaning to the result — and or or do not
	// care — but without it a rewritten tree comes back in whatever order the table
	// felt like, and a person would watch their condition rearrange itself.
	Position int    `gorm:"not null"`
	Operator string `gorm:"size:8;not null;default:''"`
	// SourceLabel and ExpectedSignal are the comparison. Empty on a group.
	SourceLabel    string `gorm:"size:32;not null;default:''"`
	ExpectedSignal string `gorm:"size:16;not null;default:''"`

	Parent   *StrategyBotConditionNode  `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
	Children []StrategyBotConditionNode `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to StrategyBotConditionNodes instead of GORM's default.
func (strategyBotConditionNode StrategyBotConditionNode) TableName() string {
	return "StrategyBotConditionNodes"
}

// IsGroup says whether this node joins other conditions rather than comparing a
// signal. It reads the one field that decides, so that no caller has to remember
// which of the three columns is the discriminator.
func (strategyBotConditionNode StrategyBotConditionNode) IsGroup() bool {
	return strategyBotConditionNode.Operator != ""
}
