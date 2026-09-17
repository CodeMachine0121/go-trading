package dto

// StrategyScriptParameterWriteDto is one knob as it was declared, before anything about it
// has been settled: the name still carries whatever blanks were typed around it, and
// the kind is still just the spelling that arrived.
type StrategyScriptParameterWriteDto struct {
	Name         string
	Kind         string
	DefaultValue float64
}
