package vo

// ConditionOperatorVo is how one condition group joins the conditions inside it.
//
// There are two and they stay two. A third — "at least N of them" — is a different
// question that would need a number alongside it, and inventing a place for that
// number before anybody asks for it would put an unused field on every group.
// Immutable, no behavior: what each one does to a set of answers lives in
// TradingStrategyConditionDomain.
type ConditionOperatorVo string

const (
	// ConditionOperatorAnd holds only when every condition inside holds.
	ConditionOperatorAnd ConditionOperatorVo = "and"
	// ConditionOperatorOr holds as soon as any condition inside holds.
	ConditionOperatorOr ConditionOperatorVo = "or"
)
