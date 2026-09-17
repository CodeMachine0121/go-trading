package dto

// StrategyScriptParameterValueDto is what one run says a knob is worth this time.
//
// It carries no kind. Which kind a name was declared as is the strategy script's word, not
// the caller's, so supplying a value cannot change what that name means.
// The tags are what makes this readable by a client, and they are not optional
// decoration: this shape goes out inside a strategy bot, where its absence wrote Name
// and Value while every caller asks for name and value. A reader that finds neither
// gets nothing, hands nothing back on the next write, and the write is then refused
// for naming a knob the strategy script never declared — a failure that surfaces three steps
// away from the field that caused it.
type StrategyScriptParameterValueDto struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}
