package dto

// TradingStrategyWriteDto is a trading strategy as it arrives to be saved, before
// anything about it has been settled: the name still carries whatever blanks were
// typed around it, and the operators and signals are still just the spellings that
// came in.
//
// Creating and rewriting hand over the same shape, because every rule that applies
// to one applies word for word to the other. ID is zero on a create and names the
// trading strategy on a rewrite; OwnerID is settled by whoever is signed in and is
// never read from the request.
type TradingStrategyWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string

	SignalSources []TradingStrategySignalSourceWriteDto
	BuyCondition  TradingStrategyConditionDto
	SellCondition TradingStrategyConditionDto
}

// TradingStrategySignalSourceWriteDto is one signal source as it arrives, plus the
// two things only the strategy script itself can say: which knobs it declares, and
// what kind of value it produces.
//
// DeclaredParameters and DeclaredResultType are filled in by the application from the
// resolved strategy script, not by the caller. They are here so that "this source
// sets a knob that strategy script never declared" and "this source listens to a
// strategy script that never speaks in signals" are both caught by the same model
// that checks everything else about a source, rather than surfacing much later as a
// script failure in the middle of the night.
type TradingStrategySignalSourceWriteDto struct {
	Label               string
	StrategyScriptID    uint
	AggregationInterval string
	ParameterValues     []StrategyScriptParameterValueDto
	DeclaredParameters  []StrategyScriptParameterWriteDto
	// DeclaredResultType is the kind of value the named strategy script declares.
	// A condition compares a source against buy, sell or hold, and only one kind of
	// strategy script ever produces those — so this is the only field that decides
	// whether a source can mean anything at all.
	DeclaredResultType string
}
