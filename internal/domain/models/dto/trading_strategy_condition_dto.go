package dto

// TradingStrategyConditionDto is a nested tree (stored flat); an empty Operator marks a
// comparison, otherwise a group.
type TradingStrategyConditionDto struct {
	// Operator is "and" or "or" on a group and empty on a comparison.
	Operator    string                        `json:"operator,omitempty"`
	Conditions  []TradingStrategyConditionDto `json:"conditions,omitempty"`
	SourceLabel string                        `json:"sourceLabel,omitempty"`
	// Signal is buy, sell or hold; there is no "not equal" since it is expressible with or.
	Signal string `json:"signal,omitempty"`
}
