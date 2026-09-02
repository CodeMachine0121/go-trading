package vo

// IndicatorResultTypeVo is the kind of value one calculation's indicators carry.
// A calculation declares exactly one of these; every indicator it produces holds a
// value of that kind. Immutable, no behavior — how a kind is read, defaulted and
// described lives in IndicatorResultTypeDomain.
type IndicatorResultTypeVo string

const (
	// IndicatorResultTypeFloat is one number per indicator, the kind assumed when a
	// caller declares nothing.
	IndicatorResultTypeFloat IndicatorResultTypeVo = "float"
	// IndicatorResultTypeFloatList is a series of numbers per indicator.
	IndicatorResultTypeFloatList IndicatorResultTypeVo = "floatList"
	// IndicatorResultTypeBool is one true/false answer per indicator.
	IndicatorResultTypeBool IndicatorResultTypeVo = "bool"
	// IndicatorResultTypeBoolList is a series of true/false answers per indicator.
	IndicatorResultTypeBoolList IndicatorResultTypeVo = "boolList"
)
