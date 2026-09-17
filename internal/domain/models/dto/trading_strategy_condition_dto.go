package dto

// TradingStrategyConditionDto is one condition as it travels in and out of the domain:
// either a comparison against one signal source, or a group of conditions joined by
// an operator.
//
// It is nested rather than a flat list of nodes with parent references. A flat list
// is how the tree is stored, because that is what a table can hold; handing one out
// would make every reader — including the screen a person builds the condition on —
// reassemble the tree before it could show anything, and two reassemblers
// eventually disagree.
//
// One shape carries both kinds instead of two shapes, because a condition is one
// thing from the caller's side: the operator being empty is what says this one is a
// comparison. Which fields are filled is settled in TradingStrategyConditionDomain, so
// nothing downstream has to guess.
type TradingStrategyConditionDto struct {
	// Operator is "and" or "or" on a group, and empty on a comparison.
	Operator string `json:"operator,omitempty"`
	// Conditions are what a group joins. Empty on a comparison.
	Conditions []TradingStrategyConditionDto `json:"conditions,omitempty"`
	// SourceLabel is which signal source a comparison reads. Empty on a group.
	SourceLabel string `json:"sourceLabel,omitempty"`
	// Signal is the value that source must equal for this comparison to hold: buy,
	// sell or hold. Empty on a group.
	//
	// There is no "not equal". A signal has three values, so "not buy" is written
	// "sell or hold" and loses nothing — while a second way of saying the same
	// thing would mean every screen and every reader has to handle both.
	Signal string `json:"signal,omitempty"`
}
