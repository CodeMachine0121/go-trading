package dto

// StrategyParameterValueDto is what one run says a knob is worth this time.
//
// It carries no kind. Which kind a name was declared as is the strategy's word, not
// the caller's, so supplying a value cannot change what that name means.
type StrategyParameterValueDto struct {
	Name  string
	Value float64
}
