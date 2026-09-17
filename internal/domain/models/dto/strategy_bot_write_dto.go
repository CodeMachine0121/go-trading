package dto

// StrategyBotWriteDto is a strategy bot as it arrives to be saved, before anything
// about it has been settled: the name still carries whatever blanks were typed
// around it, and the operators and signals are still just the spellings that came in.
//
// Creating and rewriting hand over the same shape, because every rule that applies
// to one applies word for word to the other. ID is zero on a create and names the
// bot on a rewrite; OwnerID is settled by whoever is signed in and is never read
// from the request.
type StrategyBotWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	Symbol  string

	TriggerIntervalMinutes int
	SignalSources          []StrategyBotSignalSourceWriteDto
	BuyCondition           StrategyBotConditionDto
	SellCondition          StrategyBotConditionDto
}

// StrategyBotSignalSourceWriteDto is one signal source as it arrives, plus the one
// thing only the strategy script itself can say: which knobs it declares.
//
// DeclaredParameters is filled in by the application from the resolved strategy script, not
// by the caller. It is here so that "this bot sets a knob that strategy script never
// declared" is caught by the same model that checks everything else about a source,
// rather than surfacing much later as a script failure in the middle of the night.
type StrategyBotSignalSourceWriteDto struct {
	Label               string
	StrategyScriptID    uint
	AggregationInterval string
	ParameterValues     []StrategyScriptParameterValueDto
	DeclaredParameters  []StrategyScriptParameterWriteDto
}
